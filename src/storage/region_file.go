package storage

import (
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"sync"
)

// A region file stores the chunks of a 32x32 chunk area:
//
//	header:  magic "GMCR", version u16, reserved u16,
//	         1024 entries of (offset u32, length u32), zero when absent
//	records: crc32 u32 of the data, followed by length-4 bytes of data
//
// Records are only ever appended and the header entry is updated after the
// record is written, so a crash never leaves an entry pointing at a partial
// record. Once superseded records take up more space than live ones, the file
// is compacted into a copy that atomically replaces it.
const (
	regionMagic      = "GMCR"
	regionVersion    = 1
	regionSide       = 32
	regionEntries    = regionSide * regionSide
	regionHeaderSize = 8 + regionEntries*8

	// minCompactWaste is how much superseded data a region file may hold
	// before it is compacted.
	minCompactWaste = 1 << 20
)

type regionFile struct {
	path    string
	mu      sync.Mutex
	f       *os.File
	entries [regionEntries][2]uint32
	end     int64
}

func openRegionFile(path string) (*regionFile, error) {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o644)

	if err != nil {
		return nil, err
	}

	r := &regionFile{path: path, f: f}

	if err = r.init(); err != nil {
		f.Close()
		return nil, fmt.Errorf("storage: %s: %w", path, err)
	}

	return r, nil
}

func (r *regionFile) init() error {
	info, err := r.f.Stat()

	if err != nil {
		return err
	}

	header := make([]byte, regionHeaderSize)

	if info.Size() == 0 {
		copy(header, regionMagic)
		binary.BigEndian.PutUint16(header[4:], regionVersion)

		if _, err = r.f.WriteAt(header, 0); err != nil {
			return err
		}

		r.end = regionHeaderSize

		return nil
	}

	if _, err = io.ReadFull(io.NewSectionReader(r.f, 0, regionHeaderSize), header); err != nil {
		return err
	}

	if string(header[:4]) != regionMagic || binary.BigEndian.Uint16(header[4:]) != regionVersion {
		return errors.New("not a region file of a supported version")
	}

	for i := range r.entries {
		r.entries[i][0] = binary.BigEndian.Uint32(header[8+i*8:])
		r.entries[i][1] = binary.BigEndian.Uint32(header[12+i*8:])
	}

	r.end = info.Size()

	return nil
}

// has reports whether data is stored at index.
func (r *regionFile) has(index int) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.entries[index][0] != 0
}

// read returns the data stored at index, or nil if there is none.
func (r *regionFile) read(index int) ([]byte, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	offset, length := r.entries[index][0], r.entries[index][1]

	if offset == 0 {
		return nil, nil
	}

	if length < 4 {
		return nil, errors.New("storage: corrupt region entry")
	}

	record := make([]byte, length)

	if _, err := r.f.ReadAt(record, int64(offset)); err != nil {
		return nil, err
	}

	data := record[4:]

	if crc32.ChecksumIEEE(data) != binary.BigEndian.Uint32(record) {
		return nil, errors.New("storage: chunk record checksum mismatch")
	}

	return data, nil
}

// write appends data as a new record for index and points the header at it.
func (r *regionFile) write(index int, data []byte) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.end+int64(len(data))+4 > 1<<32-1 {
		return errors.New("storage: region file is full")
	}

	record := make([]byte, 4+len(data))
	binary.BigEndian.PutUint32(record, crc32.ChecksumIEEE(data))
	copy(record[4:], data)

	offset := r.end

	if _, err := r.f.WriteAt(record, offset); err != nil {
		return err
	}

	var entry [8]byte
	binary.BigEndian.PutUint32(entry[0:], uint32(offset))
	binary.BigEndian.PutUint32(entry[4:], uint32(len(record)))

	if _, err := r.f.WriteAt(entry[:], int64(8+index*8)); err != nil {
		return err
	}

	r.entries[index] = [2]uint32{uint32(offset), uint32(len(record))}
	r.end += int64(len(record))

	var live int64

	for _, e := range r.entries {
		live += int64(e[1])
	}

	if waste := r.end - regionHeaderSize - live; waste > max(live, minCompactWaste) {
		return r.compact()
	}

	return nil
}

// compact rewrites the file without superseded records. The copy is synced
// before it replaces the original, so a crash leaves one of the two intact.
func (r *regionFile) compact() error {
	tmp := r.path + ".tmp"
	f, err := os.Create(tmp)

	if err != nil {
		return err
	}

	defer os.Remove(tmp)

	header := make([]byte, regionHeaderSize)
	copy(header, regionMagic)
	binary.BigEndian.PutUint16(header[4:], regionVersion)

	var entries [regionEntries][2]uint32
	end := int64(regionHeaderSize)

	for i, e := range r.entries {
		if e[0] == 0 {
			continue
		}

		record := make([]byte, e[1])

		if _, err = r.f.ReadAt(record, int64(e[0])); err == nil {
			_, err = f.WriteAt(record, end)
		}

		if err != nil {
			f.Close()
			return err
		}

		entries[i] = [2]uint32{uint32(end), e[1]}
		binary.BigEndian.PutUint32(header[8+i*8:], uint32(end))
		binary.BigEndian.PutUint32(header[12+i*8:], e[1])
		end += int64(e[1])
	}

	if _, err = f.WriteAt(header, 0); err == nil {
		err = f.Sync()
	}

	if err == nil {
		err = os.Rename(tmp, r.path)
	}

	if err != nil {
		f.Close()
		return err
	}

	r.f.Close()
	r.f, r.entries, r.end = f, entries, end

	return nil
}

func (r *regionFile) close() error {
	return r.f.Close()
}
