package nbt

import (
	"reflect"
	"testing"
)

func TestRoundTrip(t *testing.T) {
	var c Compound
	c.Set("text", String("hello \x00 wörld 🎮"))
	c.Set("bold", Byte(1))
	c.Set("count", Int(-7))
	c.Set("big", Long(1<<40))
	c.Set("ratio", Float(0.5))
	c.Set("precise", Double(-1.25))
	c.Set("raw", ByteArray{1, 2})
	c.Set("ints", IntArray{1, -2})
	c.Set("longs", LongArray{3, -4})
	c.Set("extra", NewList(String("a"), String("b")))
	c.Set("empty", NewList())

	encoded := Append(nil, c)
	decoded, n, err := Decode(append(encoded, 0xAA))

	if err != nil {
		t.Fatal(err)
	}

	if n != len(encoded) {
		t.Fatalf("consumed %d bytes, want %d", n, len(encoded))
	}

	if !reflect.DeepEqual(decoded, c) {
		t.Fatalf("decoded %#v, want %#v", decoded, c)
	}
}

func TestModifiedUTF8(t *testing.T) {
	encoded := Append(nil, String("\x00"))

	// Type byte, two length bytes, then the two byte encoding of NUL.
	if len(encoded) != 5 || encoded[3] != 0xC0 || encoded[4] != 0x80 {
		t.Fatalf("NUL encoded as %x", encoded)
	}
}

func TestTruncated(t *testing.T) {
	encoded := Append(nil, Compound{{Name: "a", Value: Int(1)}})

	if _, _, err := Decode(encoded[:len(encoded)-2]); err == nil {
		t.Fatal("expected an error for truncated input")
	}
}
