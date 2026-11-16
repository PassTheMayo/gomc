package protocol

import "math"

// Low precision vectors pack three components into 15 bits each plus a shared
// integer scale, used for entity velocities.
const (
	lpMinValue    = 3.051944088384301e-05
	lpMaxValue    = 1.7179869183e10
	lpMaxQuantize = 32766
	lpScaleMask   = 3
	lpContinue    = 4
)

// LpVec3 writes a low precision vector.
func (w *Writer) LpVec3(x, y, z float64) {
	x, y, z = lpSanitize(x), lpSanitize(y), lpSanitize(z)
	m := math.Max(math.Abs(x), math.Max(math.Abs(y), math.Abs(z)))

	if m < lpMinValue {
		w.UByte(0)
		return
	}

	scale := int64(math.Ceil(m))
	extended := scale&lpScaleMask != scale
	header := scale

	if extended {
		header = scale&lpScaleMask | lpContinue
	}

	packed := header | lpPack(x/float64(scale))<<3 | lpPack(y/float64(scale))<<18 | lpPack(z/float64(scale))<<33

	w.UByte(uint8(packed))
	w.UByte(uint8(packed >> 8))
	w.Int(int32(packed >> 16))

	if extended {
		w.VarInt(int32(scale >> 2))
	}
}

// LpVec3 reads a low precision vector.
func (r *Reader) LpVec3() (x, y, z float64) {
	b1 := r.UByte()

	if b1 == 0 {
		return 0, 0, 0
	}

	b2 := r.UByte()
	packed := int64(uint32(r.Int()))<<16 | int64(b2)<<8 | int64(b1)
	scale := int64(b1 & lpScaleMask)

	if b1&lpContinue != 0 {
		scale |= int64(uint32(r.VarInt())) << 2
	}

	s := float64(scale)

	return lpUnpack(packed>>3) * s, lpUnpack(packed>>18) * s, lpUnpack(packed>>33) * s
}

func lpSanitize(v float64) float64 {
	if math.IsNaN(v) {
		return 0
	}

	return math.Max(-lpMaxValue, math.Min(v, lpMaxValue))
}

func lpPack(v float64) int64 {
	return int64(math.Round((v*0.5 + 0.5) * lpMaxQuantize))
}

func lpUnpack(v int64) float64 {
	return math.Min(float64(v&32767), lpMaxQuantize)*2/lpMaxQuantize - 1
}
