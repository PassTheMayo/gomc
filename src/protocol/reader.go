package protocol

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"unicode/utf8"

	"main/src/uuid"
)

// ErrTrailingData is returned when a packet contains bytes after its last field.
var ErrTrailingData = errors.New("protocol: trailing data after packet")

// Reader decodes the payload of a packet. The first error encountered is
// remembered and every following read becomes a no-op returning a zero value,
// so decoders can read all fields and check Err once at the end.
type Reader struct {
	buf []byte
	off int
	err error
}

// NewReader returns a Reader over b.
func NewReader(b []byte) *Reader {
	return &Reader{buf: b}
}

// Err returns the first error encountered while reading.
func (r *Reader) Err() error {
	return r.err
}

// Fail records err if no previous error was recorded.
func (r *Reader) Fail(err error) {
	if r.err == nil {
		r.err = err
	}
}

// Remaining returns the number of unread bytes.
func (r *Reader) Remaining() int {
	return len(r.buf) - r.off
}

// Finish returns the first read error, or ErrTrailingData if unread bytes remain.
func (r *Reader) Finish() error {
	if r.err == nil && r.Remaining() > 0 {
		r.err = ErrTrailingData
	}

	return r.err
}

func (r *Reader) take(n int) []byte {
	if r.err != nil {
		return nil
	}

	if n < 0 || r.Remaining() < n {
		r.err = io.ErrUnexpectedEOF
		return nil
	}

	b := r.buf[r.off : r.off+n]
	r.off += n

	return b
}

// ReadByte implements io.ByteReader.
func (r *Reader) ReadByte() (byte, error) {
	b := r.take(1)

	if b == nil {
		return 0, r.err
	}

	return b[0], nil
}

// Raw reads n bytes without a length prefix.
func (r *Reader) Raw(n int) []byte {
	b := r.take(n)

	if b == nil {
		return nil
	}

	return append([]byte(nil), b...)
}

// Rest reads every remaining byte.
func (r *Reader) Rest() []byte {
	return r.Raw(r.Remaining())
}

// Unread returns the unread bytes without consuming them.
func (r *Reader) Unread() []byte {
	if r.err != nil {
		return nil
	}

	return r.buf[r.off:]
}

// Skip consumes n bytes.
func (r *Reader) Skip(n int) {
	r.take(n)
}

// Bool reads a boolean.
func (r *Reader) Bool() bool {
	b := r.take(1)

	return b != nil && b[0] != 0
}

// Byte reads a signed byte.
func (r *Reader) Byte() int8 {
	return int8(r.UByte())
}

// UByte reads an unsigned byte.
func (r *Reader) UByte() uint8 {
	b := r.take(1)

	if b == nil {
		return 0
	}

	return b[0]
}

// Short reads a big-endian signed 16-bit integer.
func (r *Reader) Short() int16 {
	return int16(r.UShort())
}

// UShort reads a big-endian unsigned 16-bit integer.
func (r *Reader) UShort() uint16 {
	b := r.take(2)

	if b == nil {
		return 0
	}

	return binary.BigEndian.Uint16(b)
}

// Int reads a big-endian signed 32-bit integer.
func (r *Reader) Int() int32 {
	b := r.take(4)

	if b == nil {
		return 0
	}

	return int32(binary.BigEndian.Uint32(b))
}

// Long reads a big-endian signed 64-bit integer.
func (r *Reader) Long() int64 {
	b := r.take(8)

	if b == nil {
		return 0
	}

	return int64(binary.BigEndian.Uint64(b))
}

// Float reads a big-endian IEEE 754 single precision float.
func (r *Reader) Float() float32 {
	return math.Float32frombits(uint32(r.Int()))
}

// Double reads a big-endian IEEE 754 double precision float.
func (r *Reader) Double() float64 {
	return math.Float64frombits(uint64(r.Long()))
}

// VarInt reads a variable-length 32-bit integer.
func (r *Reader) VarInt() int32 {
	if r.err != nil {
		return 0
	}

	v, err := ReadVarInt(r)

	if err != nil {
		r.Fail(err)
		return 0
	}

	return v
}

// VarLong reads a variable-length 64-bit integer.
func (r *Reader) VarLong() int64 {
	if r.err != nil {
		return 0
	}

	v, err := ReadVarLong(r)

	if err != nil {
		r.Fail(err)
		return 0
	}

	return v
}

// Length reads a VarInt length prefix and validates it against max and the
// remaining payload size.
func (r *Reader) Length(max int) int {
	n := int(r.VarInt())

	if r.err != nil {
		return 0
	}

	if n < 0 || n > max {
		r.Fail(fmt.Errorf("protocol: length %d out of range [0, %d]", n, max))
		return 0
	}

	return n
}

// String reads a VarInt length-prefixed UTF-8 string of at most maxChars characters.
func (r *Reader) String(maxChars int) string {
	n := r.Length(maxChars * 3)
	b := r.take(n)

	if b == nil {
		return ""
	}

	if !utf8.Valid(b) {
		r.Fail(errors.New("protocol: string is not valid UTF-8"))
		return ""
	}

	if utf8.RuneCount(b) > maxChars {
		r.Fail(fmt.Errorf("protocol: string longer than %d characters", maxChars))
		return ""
	}

	return string(b)
}

// ByteArray reads a VarInt length-prefixed byte array of at most max bytes.
func (r *Reader) ByteArray(max int) []byte {
	return r.Raw(r.Length(max))
}

// UUID reads a UUID encoded as two big-endian 64-bit integers.
func (r *Reader) UUID() uuid.UUID {
	var u uuid.UUID

	copy(u[:], r.take(16))

	return u
}

// FixedLongArray reads n longs without a length prefix.
func (r *Reader) FixedLongArray(n int) []uint64 {
	b := r.take(n * 8)

	if b == nil {
		return nil
	}

	out := make([]uint64, n)

	for i := range out {
		out[i] = binary.BigEndian.Uint64(b[i*8:])
	}

	return out
}

// LongArray reads a VarInt length-prefixed array of at most max longs.
func (r *Reader) LongArray(max int) []uint64 {
	return r.FixedLongArray(r.Length(max))
}

// BitSet reads a VarInt length-prefixed bit set.
func (r *Reader) BitSet(maxBytes int) BitSet {
	return BitSetFromBytes(r.ByteArray(maxBytes))
}

// Position reads a block position packed into a single long.
func (r *Reader) Position() (x, y, z int32) {
	return UnpackBlockPos(r.Long())
}

// Angle reads a rotation byte and returns it in degrees.
func (r *Reader) Angle() float32 {
	return ByteToAngle(r.UByte())
}
