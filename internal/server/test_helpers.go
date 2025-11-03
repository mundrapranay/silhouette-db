package server

import (
	"encoding/binary"
	"math"
)

// float64ToBytes converts float64 to 8-byte little-endian bytes
func float64ToBytes(f float64) []byte {
	bytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(bytes, math.Float64bits(f))
	return bytes
}

// bytesToFloat64 converts 8-byte little-endian bytes to float64
func bytesToFloat64(bytes []byte) float64 {
	if len(bytes) < 8 {
		return 0.0
	}
	bits := binary.LittleEndian.Uint64(bytes[:8])
	return math.Float64frombits(bits)
}
