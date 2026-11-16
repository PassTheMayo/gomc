package nbt

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"unicode/utf16"
)

// MaxDepth is the maximum nesting depth accepted when decoding.
const MaxDepth = 512

// ErrTruncated is returned when the input ends in the middle of a tag.
var ErrTruncated = errors.New("nbt: truncated input")

type decoder struct {
	b   []byte
	off int
}

// Decode decodes one network-encoded tag from the start of b and returns it
// with the number of bytes consumed.
func Decode(b []byte) (Tag, int, error) {
	d := &decoder{b: b}

	typ, err := d.byte()

	if err != nil {
		return nil, 0, err
	}

	tag, err := d.payload(typ, 0)

	if err != nil {
		return nil, 0, err
	}

	return tag, d.off, nil
}

func (d *decoder) take(n int) ([]byte, error) {
	if n < 0 || len(d.b)-d.off < n {
		return nil, ErrTruncated
	}

	b := d.b[d.off : d.off+n]
	d.off += n

	return b, nil
}

func (d *decoder) byte() (byte, error) {
	b, err := d.take(1)

	if err != nil {
		return 0, err
	}

	return b[0], nil
}

func (d *decoder) uint32() (uint32, error) {
	b, err := d.take(4)

	if err != nil {
		return 0, err
	}

	return binary.BigEndian.Uint32(b), nil
}

func (d *decoder) length(elemSize int) (int, error) {
	n, err := d.uint32()

	if err != nil {
		return 0, err
	}

	if int64(int32(n))*int64(elemSize) > int64(len(d.b)-d.off) || int32(n) < 0 {
		return 0, ErrTruncated
	}

	return int(n), nil
}

func (d *decoder) payload(typ byte, depth int) (Tag, error) {
	if depth > MaxDepth {
		return nil, errors.New("nbt: maximum depth exceeded")
	}

	switch typ {
	case TypeByte:
		v, err := d.byte()
		return Byte(v), err
	case TypeShort:
		b, err := d.take(2)

		if err != nil {
			return nil, err
		}

		return Short(binary.BigEndian.Uint16(b)), nil
	case TypeInt:
		v, err := d.uint32()
		return Int(v), err
	case TypeLong:
		b, err := d.take(8)

		if err != nil {
			return nil, err
		}

		return Long(binary.BigEndian.Uint64(b)), nil
	case TypeFloat:
		v, err := d.uint32()
		return Float(math.Float32frombits(v)), err
	case TypeDouble:
		b, err := d.take(8)

		if err != nil {
			return nil, err
		}

		return Double(math.Float64frombits(binary.BigEndian.Uint64(b))), nil
	case TypeByteArray:
		n, err := d.length(1)

		if err != nil {
			return nil, err
		}

		b, _ := d.take(n)

		return ByteArray(append([]byte(nil), b...)), nil
	case TypeString:
		s, err := d.string()
		return String(s), err
	case TypeList:
		elem, err := d.byte()

		if err != nil {
			return nil, err
		}

		n, err := d.length(1)

		if err != nil {
			return nil, err
		}

		list := List{ElemType: elem}

		for i := 0; i < n; i++ {
			item, err := d.payload(elem, depth+1)

			if err != nil {
				return nil, err
			}

			list.Items = append(list.Items, item)
		}

		return list, nil
	case TypeCompound:
		var c Compound

		for {
			t, err := d.byte()

			if err != nil {
				return nil, err
			}

			if t == TypeEnd {
				return c, nil
			}

			name, err := d.string()

			if err != nil {
				return nil, err
			}

			v, err := d.payload(t, depth+1)

			if err != nil {
				return nil, err
			}

			c = append(c, Entry{Name: name, Value: v})
		}
	case TypeIntArray:
		n, err := d.length(4)

		if err != nil {
			return nil, err
		}

		out := make(IntArray, n)

		for i := range out {
			v, _ := d.uint32()
			out[i] = int32(v)
		}

		return out, nil
	case TypeLongArray:
		n, err := d.length(8)

		if err != nil {
			return nil, err
		}

		out := make(LongArray, n)

		for i := range out {
			b, _ := d.take(8)
			out[i] = int64(binary.BigEndian.Uint64(b))
		}

		return out, nil
	}

	return nil, fmt.Errorf("nbt: unknown tag type %d", typ)
}

// string reads a modified UTF-8 string with a 16-bit length prefix.
func (d *decoder) string() (string, error) {
	lb, err := d.take(2)

	if err != nil {
		return "", err
	}

	b, err := d.take(int(binary.BigEndian.Uint16(lb)))

	if err != nil {
		return "", err
	}

	units := make([]uint16, 0, len(b))

	for i := 0; i < len(b); {
		c := b[i]

		switch {
		case c < 0x80:
			units = append(units, uint16(c))
			i++
		case c&0xE0 == 0xC0 && i+1 < len(b):
			units = append(units, uint16(c&0x1F)<<6|uint16(b[i+1]&0x3F))
			i += 2
		case c&0xF0 == 0xE0 && i+2 < len(b):
			units = append(units, uint16(c&0x0F)<<12|uint16(b[i+1]&0x3F)<<6|uint16(b[i+2]&0x3F))
			i += 3
		default:
			return "", errors.New("nbt: malformed modified UTF-8")
		}
	}

	return string(utf16.Decode(units)), nil
}
