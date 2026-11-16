package protocol

import (
	"encoding/binary"
	"math"

	"main/src/uuid"
)

// Writer builds the payload of a packet.
type Writer struct {
	buf []byte
}

// NewWriter returns an empty Writer.
func NewWriter() *Writer {
	return &Writer{buf: make([]byte, 0, 64)}
}

// Bytes returns the bytes written so far.
func (w *Writer) Bytes() []byte {
	return w.buf
}

// Raw writes b without a length prefix.
func (w *Writer) Raw(b []byte) {
	w.buf = append(w.buf, b...)
}

// Bool writes a boolean as a single byte.
func (w *Writer) Bool(v bool) {
	if v {
		w.buf = append(w.buf, 1)
	} else {
		w.buf = append(w.buf, 0)
	}
}

// Byte writes a signed byte.
func (w *Writer) Byte(v int8) {
	w.buf = append(w.buf, byte(v))
}

// UByte writes an unsigned byte.
func (w *Writer) UByte(v uint8) {
	w.buf = append(w.buf, v)
}

// Short writes a big-endian signed 16-bit integer.
func (w *Writer) Short(v int16) {
	w.buf = binary.BigEndian.AppendUint16(w.buf, uint16(v))
}

// UShort writes a big-endian unsigned 16-bit integer.
func (w *Writer) UShort(v uint16) {
	w.buf = binary.BigEndian.AppendUint16(w.buf, v)
}

// Int writes a big-endian signed 32-bit integer.
func (w *Writer) Int(v int32) {
	w.buf = binary.BigEndian.AppendUint32(w.buf, uint32(v))
}

// Long writes a big-endian signed 64-bit integer.
func (w *Writer) Long(v int64) {
	w.buf = binary.BigEndian.AppendUint64(w.buf, uint64(v))
}

// Float writes a big-endian IEEE 754 single precision float.
func (w *Writer) Float(v float32) {
	w.buf = binary.BigEndian.AppendUint32(w.buf, math.Float32bits(v))
}

// Double writes a big-endian IEEE 754 double precision float.
func (w *Writer) Double(v float64) {
	w.buf = binary.BigEndian.AppendUint64(w.buf, math.Float64bits(v))
}

// VarInt writes a variable-length 32-bit integer.
func (w *Writer) VarInt(v int32) {
	w.buf = AppendVarInt(w.buf, v)
}

// VarLong writes a variable-length 64-bit integer.
func (w *Writer) VarLong(v int64) {
	w.buf = AppendVarLong(w.buf, v)
}

// String writes a VarInt length-prefixed UTF-8 string.
func (w *Writer) String(v string) {
	w.VarInt(int32(len(v)))
	w.buf = append(w.buf, v...)
}

// ByteArray writes a VarInt length-prefixed byte array.
func (w *Writer) ByteArray(v []byte) {
	w.VarInt(int32(len(v)))
	w.buf = append(w.buf, v...)
}

// UUID writes a UUID as two big-endian 64-bit integers.
func (w *Writer) UUID(v uuid.UUID) {
	w.buf = append(w.buf, v[:]...)
}

// FixedLongArray writes longs without a length prefix.
func (w *Writer) FixedLongArray(v []uint64) {
	for _, l := range v {
		w.buf = binary.BigEndian.AppendUint64(w.buf, l)
	}
}

// LongArray writes a VarInt length-prefixed array of longs.
func (w *Writer) LongArray(v []uint64) {
	w.VarInt(int32(len(v)))
	w.FixedLongArray(v)
}

// BitSet writes a bit set using the layout of Java's BitSet.toByteArray,
// prefixed by its VarInt byte length.
func (w *Writer) BitSet(v BitSet) {
	w.ByteArray(v.Bytes())
}

// Position writes a block position packed into a single long.
func (w *Writer) Position(x, y, z int32) {
	w.Long(PackBlockPos(x, y, z))
}

// Angle writes a rotation in degrees as a single byte of 1/256th turns.
func (w *Writer) Angle(degrees float32) {
	w.buf = append(w.buf, AngleToByte(degrees))
}
