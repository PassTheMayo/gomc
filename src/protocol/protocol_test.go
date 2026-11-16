package protocol

import (
	"bytes"
	"encoding/hex"
	"math"
	"testing"

	"main/src/uuid"
)

func TestVarIntEncoding(t *testing.T) {
	cases := map[int32]string{
		0:           "00",
		1:           "01",
		127:         "7f",
		128:         "8001",
		255:         "ff01",
		25565:       "ddc701",
		2097151:     "ffff7f",
		2147483647:  "ffffffff07",
		-1:          "ffffffff0f",
		-2147483648: "8080808008",
	}

	for v, want := range cases {
		got := hex.EncodeToString(AppendVarInt(nil, v))

		if got != want {
			t.Errorf("AppendVarInt(%d) = %s, want %s", v, got, want)
		}

		if VarIntSize(v) != len(want)/2 {
			t.Errorf("VarIntSize(%d) = %d, want %d", v, VarIntSize(v), len(want)/2)
		}

		decoded, err := ReadVarInt(bytes.NewReader(AppendVarInt(nil, v)))

		if err != nil || decoded != v {
			t.Errorf("ReadVarInt(%s) = %d, %v", want, decoded, err)
		}
	}

	if _, err := ReadVarInt(bytes.NewReader([]byte{0xff, 0xff, 0xff, 0xff, 0xff, 0x01})); err != ErrVarIntTooBig {
		t.Errorf("expected ErrVarIntTooBig, got %v", err)
	}
}

func TestVarLongRoundTrip(t *testing.T) {
	for _, v := range []int64{0, 1, -1, math.MaxInt64, math.MinInt64, 1 << 40} {
		got, err := ReadVarLong(bytes.NewReader(AppendVarLong(nil, v)))

		if err != nil || got != v {
			t.Errorf("VarLong round trip of %d = %d, %v", v, got, err)
		}
	}
}

func TestWriterReaderRoundTrip(t *testing.T) {
	id := uuid.New()
	w := NewWriter()
	w.Bool(true)
	w.Byte(-5)
	w.Short(-300)
	w.UShort(65000)
	w.Int(-123456)
	w.Long(math.MinInt64)
	w.Float(1.5)
	w.Double(-2.25)
	w.String("héllo ✓")
	w.ByteArray([]byte{1, 2, 3})
	w.UUID(id)
	w.Position(-1000, -64, 2000)
	w.LongArray([]uint64{7, 8})

	r := NewReader(w.Bytes())

	if !r.Bool() || r.Byte() != -5 || r.Short() != -300 || r.UShort() != 65000 || r.Int() != -123456 ||
		r.Long() != math.MinInt64 || r.Float() != 1.5 || r.Double() != -2.25 || r.String(16) != "héllo ✓" {
		t.Fatal("primitive round trip mismatch")
	}

	if !bytes.Equal(r.ByteArray(16), []byte{1, 2, 3}) || r.UUID() != id {
		t.Fatal("array or UUID round trip mismatch")
	}

	if x, y, z := r.Position(); x != -1000 || y != -64 || z != 2000 {
		t.Fatalf("position = %d %d %d", x, y, z)
	}

	if l := r.LongArray(4); len(l) != 2 || l[0] != 7 || l[1] != 8 {
		t.Fatalf("long array = %v", l)
	}

	if err := r.Finish(); err != nil {
		t.Fatal(err)
	}
}

func TestReaderRejectsLongStrings(t *testing.T) {
	w := NewWriter()
	w.String("seventeen chars!!")

	r := NewReader(w.Bytes())
	r.String(16)

	if r.Err() == nil {
		t.Fatal("expected an error for a string over the limit")
	}
}

func TestChunkPosPacking(t *testing.T) {
	for _, p := range [][2]int32{{0, 0}, {-1, 1}, {123456, -654321}} {
		x, z := UnpackChunkPos(PackChunkPos(p[0], p[1]))

		if x != p[0] || z != p[1] {
			t.Errorf("chunk pos round trip of %v = %d %d", p, x, z)
		}
	}

	if PackChunkPos(1, 2) != 1|2<<32 {
		t.Error("chunk X must occupy the low 32 bits")
	}
}

func TestLpVec3(t *testing.T) {
	cases := [][3]float64{{0, 0, 0}, {0.5, -0.25, 0.1}, {3.9, 0, -2}, {40, -12.5, 7}}

	for _, c := range cases {
		w := NewWriter()
		w.LpVec3(c[0], c[1], c[2])

		r := NewReader(w.Bytes())
		x, y, z := r.LpVec3()

		if err := r.Finish(); err != nil {
			t.Fatal(err)
		}

		scale := math.Max(1, math.Ceil(math.Max(math.Abs(c[0]), math.Max(math.Abs(c[1]), math.Abs(c[2])))))
		tolerance := scale * 2 / 32766

		if math.Abs(x-c[0]) > tolerance || math.Abs(y-c[1]) > tolerance || math.Abs(z-c[2]) > tolerance {
			t.Errorf("LpVec3 %v decoded as %v %v %v", c, x, y, z)
		}
	}

	w := NewWriter()
	w.LpVec3(0, 0, 0)

	if !bytes.Equal(w.Bytes(), []byte{0}) {
		t.Error("a zero vector must encode as a single zero byte")
	}
}

func TestBitSet(t *testing.T) {
	var s BitSet
	s.Set(0)
	s.Set(9)
	s.Set(25)

	if !bytes.Equal(s.Bytes(), []byte{0x01, 0x02, 0x00, 0x02}) {
		t.Errorf("BitSet bytes = %x", s.Bytes())
	}

	if !s.Get(9) || s.Get(8) || s.Get(100) {
		t.Error("BitSet.Get mismatch")
	}
}
