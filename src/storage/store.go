// Package storage persists worlds in the server's own format: region files of
// compressed chunks, plus small binary files for players and level metadata.
package storage

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"main/src/world"
)

// Store is a world directory on disk.
type Store struct {
	dir string

	mu      sync.Mutex
	regions map[[2]int32]*regionFile
}

// Open opens the world directory, creating it if needed.
func Open(dir string) (*Store, error) {
	for _, sub := range []string{"regions", "players"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o755); err != nil {
			return nil, err
		}
	}

	return &Store{dir: dir, regions: make(map[[2]int32]*regionFile)}, nil
}

func (s *Store) region(pos world.ChunkPos) (*regionFile, int, error) {
	key := [2]int32{pos.X >> 5, pos.Z >> 5}
	index := int(pos.Z&(regionSide-1))*regionSide + int(pos.X&(regionSide-1))

	s.mu.Lock()
	defer s.mu.Unlock()

	if r, ok := s.regions[key]; ok {
		return r, index, nil
	}

	path := filepath.Join(s.dir, "regions", fmt.Sprintf("r.%d.%d.region", key[0], key[1]))
	r, err := openRegionFile(path)

	if err != nil {
		return nil, 0, err
	}

	s.regions[key] = r

	return r, index, nil
}

// LoadChunk implements world.Storage.
func (s *Store) LoadChunk(pos world.ChunkPos, dim world.Dimension) (*world.Chunk, error) {
	r, index, err := s.region(pos)

	if err != nil {
		return nil, err
	}

	data, err := r.read(index)

	if err != nil || data == nil {
		return nil, err
	}

	return decodeChunk(data, pos, dim)
}

// HasChunk implements world.Storage.
func (s *Store) HasChunk(pos world.ChunkPos) (bool, error) {
	r, index, err := s.region(pos)

	if err != nil {
		return false, err
	}

	return r.has(index), nil
}

// SaveChunk implements world.Storage.
func (s *Store) SaveChunk(c *world.Chunk) error {
	data, err := encodeChunk(c)

	if err != nil {
		return err
	}

	r, index, err := s.region(c.Pos)

	if err != nil {
		return err
	}

	return r.write(index, data)
}

// Close closes every open region file.
func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var errs []error

	for key, r := range s.regions {
		errs = append(errs, r.close())
		delete(s.regions, key)
	}

	return errors.Join(errs...)
}

// writeFileAtomic replaces path with data so readers never see a partial file.
func writeFileAtomic(path string, data []byte) error {
	tmp := path + ".tmp"

	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}

	return os.Rename(tmp, path)
}
