package protocol

import "math"

// PackBlockPos packs a block position into a long: 26 bits of X, 26 bits of Z
// and 12 bits of Y, from most to least significant.
func PackBlockPos(x, y, z int32) int64 {
	return int64(x&0x3FFFFFF)<<38 | int64(z&0x3FFFFFF)<<12 | int64(y&0xFFF)
}

// UnpackBlockPos reverses PackBlockPos.
func UnpackBlockPos(v int64) (x, y, z int32) {
	x = int32(v >> 38)
	y = int32(v << 52 >> 52)
	z = int32(v << 26 >> 38)

	return x, y, z
}

// PackChunkPos packs chunk coordinates into a long with X in the low 32 bits.
func PackChunkPos(x, z int32) int64 {
	return int64(uint32(x)) | int64(uint32(z))<<32
}

// UnpackChunkPos reverses PackChunkPos.
func UnpackChunkPos(v int64) (x, z int32) {
	return int32(v), int32(v >> 32)
}

// AngleToByte converts degrees to the protocol's 1/256th turn representation.
func AngleToByte(degrees float32) byte {
	return byte(int32(math.Floor(float64(degrees) * 256.0 / 360.0)))
}

// ByteToAngle converts a 1/256th turn rotation to degrees.
func ByteToAngle(b byte) float32 {
	return float32(int8(b)) * 360.0 / 256.0
}
