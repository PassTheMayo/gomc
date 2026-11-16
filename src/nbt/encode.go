package nbt

import (
	"encoding/binary"
	"math"
	"unicode/utf16"
)

// Append appends the network encoding of tag (type byte followed by the
// unnamed payload) to b.
func Append(b []byte, tag Tag) []byte {
	b = append(b, tag.Type())

	return appendPayload(b, tag)
}

func appendPayload(b []byte, tag Tag) []byte {
	switch t := tag.(type) {
	case Byte:
		return append(b, byte(t))
	case Short:
		return binary.BigEndian.AppendUint16(b, uint16(t))
	case Int:
		return binary.BigEndian.AppendUint32(b, uint32(t))
	case Long:
		return binary.BigEndian.AppendUint64(b, uint64(t))
	case Float:
		return binary.BigEndian.AppendUint32(b, math.Float32bits(float32(t)))
	case Double:
		return binary.BigEndian.AppendUint64(b, math.Float64bits(float64(t)))
	case ByteArray:
		b = binary.BigEndian.AppendUint32(b, uint32(len(t)))
		return append(b, t...)
	case String:
		return appendString(b, string(t))
	case List:
		b = append(b, t.ElemType)
		b = binary.BigEndian.AppendUint32(b, uint32(len(t.Items)))

		for _, item := range t.Items {
			b = appendPayload(b, item)
		}

		return b
	case Compound:
		for _, e := range t {
			b = append(b, e.Value.Type())
			b = appendString(b, e.Name)
			b = appendPayload(b, e.Value)
		}

		return append(b, TypeEnd)
	case IntArray:
		b = binary.BigEndian.AppendUint32(b, uint32(len(t)))

		for _, v := range t {
			b = binary.BigEndian.AppendUint32(b, uint32(v))
		}

		return b
	case LongArray:
		b = binary.BigEndian.AppendUint32(b, uint32(len(t)))

		for _, v := range t {
			b = binary.BigEndian.AppendUint64(b, uint64(v))
		}

		return b
	}

	panic("nbt: unknown tag type")
}

// appendString appends s in Java's modified UTF-8 with a 16-bit length prefix.
func appendString(b []byte, s string) []byte {
	start := len(b)
	b = append(b, 0, 0)

	for _, r := range s {
		switch {
		case r != 0 && r < 0x80:
			b = append(b, byte(r))
		case r < 0x800:
			b = append(b, byte(0xC0|r>>6), byte(0x80|r&0x3F))
		case r < 0x10000:
			b = append(b, byte(0xE0|r>>12), byte(0x80|(r>>6)&0x3F), byte(0x80|r&0x3F))
		default:
			hi, lo := utf16.EncodeRune(r)

			for _, c := range [2]rune{hi, lo} {
				b = append(b, byte(0xE0|c>>12), byte(0x80|(c>>6)&0x3F), byte(0x80|c&0x3F))
			}
		}
	}

	binary.BigEndian.PutUint16(b[start:], uint16(len(b)-start-2))

	return b
}
