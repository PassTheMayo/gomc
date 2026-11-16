package storage

import (
	"path/filepath"

	"main/src/protocol"
)

// The level file holds: magic "GMCL", version u8, seed i64, game time i64,
// generator name as a VarInt length-prefixed string.
const (
	levelMagic   = "GMCL"
	levelVersion = 1
)

// LevelData is the persisted metadata of a world.
type LevelData struct {
	Seed      int64
	GameTime  int64
	Generator string
}

func (s *Store) levelPath() string {
	return filepath.Join(s.dir, "level.dat")
}

// LoadLevel returns the saved level metadata and whether it exists.
func (s *Store) LoadLevel() (LevelData, bool, error) {
	r, _, found, err := readVersioned(s.levelPath(), levelMagic, levelVersion)

	if !found || err != nil {
		return LevelData{}, false, err
	}

	l := LevelData{Seed: r.Long(), GameTime: r.Long(), Generator: r.String(64)}

	if err = r.Finish(); err != nil {
		return LevelData{}, false, err
	}

	return l, true, nil
}

// SaveLevel stores the level metadata.
func (s *Store) SaveLevel(l LevelData) error {
	return writeVersioned(s.levelPath(), levelMagic, levelVersion, func(w *protocol.Writer) {
		w.Long(l.Seed)
		w.Long(l.GameTime)
		w.String(l.Generator)
	})
}
