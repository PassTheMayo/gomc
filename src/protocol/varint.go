// Package protocol implements the primitive data types of the Minecraft Java
// Edition network protocol.
package protocol

import (
	"errors"
	"io"
)

const (
	// MaxVarIntLen is the maximum number of bytes a VarInt can occupy.
	MaxVarIntLen = 5

	// MaxVarLongLen is the maximum number of bytes a VarLong can occupy.
	MaxVarLongLen = 10
)

// ErrVarIntTooBig is returned when a VarInt or VarLong exceeds its maximum length.
var ErrVarIntTooBig = errors.New("protocol: varint is too big")

// AppendVarInt appends the VarInt encoding of v to b.
func AppendVarInt(b []byte, v int32) []byte {
	u := uint32(v)

	for u >= 0x80 {
		b = append(b, byte(u)|0x80)
		u >>= 7
	}

	return append(b, byte(u))
}

// AppendVarLong appends the VarLong encoding of v to b.
func AppendVarLong(b []byte, v int64) []byte {
	u := uint64(v)

	for u >= 0x80 {
		b = append(b, byte(u)|0x80)
		u >>= 7
	}

	return append(b, byte(u))
}

// VarIntSize returns the number of bytes needed to encode v as a VarInt.
func VarIntSize(v int32) int {
	u := uint32(v)
	n := 1

	for u >= 0x80 {
		u >>= 7
		n++
	}

	return n
}

// ReadVarInt reads a VarInt from r.
func ReadVarInt(r io.ByteReader) (int32, error) {
	var v uint32

	for i := 0; i < MaxVarIntLen; i++ {
		b, err := r.ReadByte()

		if err != nil {
			return 0, err
		}

		v |= uint32(b&0x7F) << (7 * i)

		if b&0x80 == 0 {
			return int32(v), nil
		}
	}

	return 0, ErrVarIntTooBig
}

// ReadVarLong reads a VarLong from r.
func ReadVarLong(r io.ByteReader) (int64, error) {
	var v uint64

	for i := 0; i < MaxVarLongLen; i++ {
		b, err := r.ReadByte()

		if err != nil {
			return 0, err
		}

		v |= uint64(b&0x7F) << (7 * i)

		if b&0x80 == 0 {
			return int64(v), nil
		}
	}

	return 0, ErrVarIntTooBig
}
