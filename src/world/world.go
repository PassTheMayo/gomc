// Package world implements chunk storage in memory, the network chunk format
// and the life cycle of loaded chunks.
package world

import (
	"errors"
	"log/slog"
	"runtime"
	"sync"
	"time"

	"main/src/block"
)

// Generator produces the terrain of new chunks.
type Generator interface {
	// Generate fills buf with the terrain of the chunk at buf.Pos.
	Generate(buf *ChunkBuffer)

	// Spawn returns the block column new players should appear in.
	Spawn() (x, z int)

	// DefaultBiome returns the biome ID new chunk buffers are filled with.
	DefaultBiome() int32
}

// Storage persists chunks.
type Storage interface {
	// LoadChunk returns the stored chunk, or nil if it was never saved.
	LoadChunk(pos ChunkPos, dim Dimension) (*Chunk, error)

	// SaveChunk stores a chunk.
	SaveChunk(c *Chunk) error

	// HasChunk reports whether a chunk was saved, without loading it.
	HasChunk(pos ChunkPos) (bool, error)
}

// World owns the loaded chunks of one dimension. Chunks are reference counted
// by the players viewing them and unloaded some time after the last viewer
// leaves.
type World struct {
	Dim     Dimension
	gen     Generator
	storage Storage
	log     *slog.Logger
	workers chan struct{}

	// BlockChanged, when set, is called for every changed block while its
	// chunk is locked, so updates sent from it are ordered with chunk packets.
	BlockChanged func(x, y, z int, s block.StateID)

	mu     sync.Mutex
	chunks map[ChunkPos]*chunkEntry
}

type chunkEntry struct {
	ready chan struct{}
	chunk *Chunk
	refs  int

	// savedVersion is the chunk version last written to storage, or -1 if
	// the chunk was never saved.
	savedVersion int64
	idleFrom     time.Time
}

// dirty reports whether the chunk changed since it was last saved.
func (e *chunkEntry) dirty() bool {
	return int64(e.chunk.Version()) != e.savedVersion
}

// New returns a world that generates missing chunks with gen and persists them
// with storage.
func New(dim Dimension, gen Generator, storage Storage, log *slog.Logger) *World {
	return &World{
		Dim:     dim,
		gen:     gen,
		storage: storage,
		log:     log,
		workers: make(chan struct{}, runtime.GOMAXPROCS(0)),
		chunks:  make(map[ChunkPos]*chunkEntry),
	}
}

// Generator returns the world's terrain generator.
func (w *World) Generator() Generator {
	return w.gen
}

// Acquire returns the chunk at pos, loading or generating it if needed, and
// keeps it loaded until a matching Release.
func (w *World) Acquire(pos ChunkPos) *Chunk {
	w.mu.Lock()
	e, ok := w.chunks[pos]

	if !ok {
		e = &chunkEntry{ready: make(chan struct{})}
		w.chunks[pos] = e
		go w.load(pos, e)
	}

	e.refs++
	w.mu.Unlock()

	// The chunk is written before ready is closed, so it can be read without the lock.
	<-e.ready

	return e.chunk
}

// Release drops a reference taken by Acquire.
func (w *World) Release(pos ChunkPos) {
	w.mu.Lock()
	defer w.mu.Unlock()

	e, ok := w.chunks[pos]

	if !ok {
		return
	}

	e.refs--

	if e.refs == 0 {
		e.idleFrom = time.Now()
	}
}

// load reads a chunk from storage or generates it. A chunk that fails to load
// is regenerated, losing any changes it had, and saved again over the
// unreadable copy. The worker semaphore bounds concurrent preparation.
func (w *World) load(pos ChunkPos, e *chunkEntry) {
	w.workers <- struct{}{}
	defer func() { <-w.workers }()

	c, err := w.storage.LoadChunk(pos, w.Dim)

	if err != nil {
		w.log.Error("Failed to load chunk, regenerating it", "x", pos.X, "z", pos.Z, "error", err)
	}

	generated := c == nil

	if generated {
		buf := NewChunkBuffer(pos, w.Dim, w.gen.DefaultBiome())
		w.gen.Generate(buf)
		c = buf.Build()
	}

	w.mu.Lock()
	e.chunk, e.savedVersion = c, 0

	if generated {
		e.savedVersion = -1
	}

	w.mu.Unlock()

	close(e.ready)
}

// loaded returns the chunk at pos if it is in memory.
func (w *World) loaded(pos ChunkPos) *Chunk {
	w.mu.Lock()
	defer w.mu.Unlock()

	if e, ok := w.chunks[pos]; ok {
		return e.chunk
	}

	return nil
}

// Block returns a block of a loaded chunk and whether that chunk is loaded.
func (w *World) Block(x, y, z int) (block.StateID, bool) {
	c := w.loaded(ChunkPos{X: int32(x >> 4), Z: int32(z >> 4)})

	if c == nil {
		return 0, false
	}

	return c.Block(x&15, y, z&15), true
}

// SetBlock changes a block of a loaded chunk and reports whether that chunk
// is loaded. The change is saved with the chunk.
func (w *World) SetBlock(x, y, z int, s block.StateID) bool {
	c := w.loaded(ChunkPos{X: int32(x >> 4), Z: int32(z >> 4)})

	if c == nil {
		return false
	}

	c.setBlock(x&15, y, z&15, s, func() {
		if w.BlockChanged != nil {
			w.BlockChanged(x, y, z, s)
		}
	})

	return true
}

// save writes a chunk and records the version that was written.
func (w *World) save(e *chunkEntry) error {
	version := int64(e.chunk.Version())

	if err := w.storage.SaveChunk(e.chunk); err != nil {
		return err
	}

	w.mu.Lock()
	e.savedVersion = max(e.savedVersion, version)
	w.mu.Unlock()

	return nil
}

// Pregenerate makes sure the chunk at pos is saved, generating it if needed,
// without keeping it in memory. It reports whether the chunk was generated.
// Loaded chunks are skipped because they are saved when they unload.
func (w *World) Pregenerate(pos ChunkPos) (bool, error) {
	w.mu.Lock()
	_, loaded := w.chunks[pos]
	w.mu.Unlock()

	if loaded {
		return false, nil
	}

	stored, err := w.storage.HasChunk(pos)

	if err != nil || stored {
		return false, err
	}

	// The entry lets players that need the chunk meanwhile share this
	// generation instead of starting their own.
	w.mu.Lock()

	if _, loaded = w.chunks[pos]; loaded {
		w.mu.Unlock()
		return false, nil
	}

	e := &chunkEntry{ready: make(chan struct{}), refs: 1}
	w.chunks[pos] = e
	w.mu.Unlock()

	w.load(pos, e)
	err = w.save(e)

	w.mu.Lock()
	defer w.mu.Unlock()

	e.refs--

	if e.refs == 0 {
		if !e.dirty() {
			delete(w.chunks, pos)
		} else {
			e.idleFrom = time.Now()
		}
	}

	return true, err
}

// UnloadIdle saves and unloads chunks nobody has viewed for at least maxIdle.
func (w *World) UnloadIdle(maxIdle time.Duration) {
	w.mu.Lock()
	var idle []*chunkEntry

	for pos, e := range w.chunks {
		if e.refs == 0 && e.chunk != nil && time.Since(e.idleFrom) >= maxIdle {
			delete(w.chunks, pos)

			if e.dirty() {
				idle = append(idle, e)
			}
		}
	}

	w.mu.Unlock()

	for _, e := range idle {
		if err := w.save(e); err != nil {
			w.log.Error("Failed to save chunk", "x", e.chunk.Pos.X, "z", e.chunk.Pos.Z, "error", err)
		}
	}
}

// LoadedChunks returns the number of chunks in memory.
func (w *World) LoadedChunks() int {
	w.mu.Lock()
	defer w.mu.Unlock()

	return len(w.chunks)
}

// Save writes every loaded chunk that changed since it was last saved.
func (w *World) Save() error {
	w.mu.Lock()
	var dirty []*chunkEntry

	for _, e := range w.chunks {
		if e.chunk != nil && e.dirty() {
			dirty = append(dirty, e)
		}
	}

	w.mu.Unlock()

	var errs []error

	for _, e := range dirty {
		errs = append(errs, w.save(e))
	}

	return errors.Join(errs...)
}
