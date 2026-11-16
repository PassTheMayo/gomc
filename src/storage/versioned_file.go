package storage

import (
	"errors"
	"fmt"
	"os"

	"main/src/protocol"
)

// readVersioned opens a file that starts with a four byte magic and a version
// byte, and returns a reader positioned after them. found is false when the
// file doesn't exist. Versions from 1 to latest are accepted.
func readVersioned(path, magic string, latest uint8) (r *protocol.Reader, version uint8, found bool, err error) {
	data, err := os.ReadFile(path)

	if errors.Is(err, os.ErrNotExist) {
		return nil, 0, false, nil
	}

	if err != nil {
		return nil, 0, false, err
	}

	r = protocol.NewReader(data)

	if string(r.Raw(len(magic))) != magic {
		return nil, 0, false, fmt.Errorf("storage: %s is not a %s file", path, magic)
	}

	if version = r.UByte(); version < 1 || version > latest {
		return nil, 0, false, fmt.Errorf("storage: %s has unsupported version %d", path, version)
	}

	return r, version, true, nil
}

// writeVersioned atomically replaces path with the magic, the version and the
// body written by write.
func writeVersioned(path, magic string, version uint8, write func(w *protocol.Writer)) error {
	w := protocol.NewWriter()
	w.Raw([]byte(magic))
	w.UByte(version)
	write(w)

	return writeFileAtomic(path, w.Bytes())
}
