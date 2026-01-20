package utils

import "math"

// ConvertFloat32ArrayToBytes converts a float32 array to bytes using little-endian encoding
func ConvertFloat32ArrayToBytes(floats []float32) []byte {
	size := len(floats) * 4
	bytes := make([]byte, size)
	for i, f := range floats {
		bits := math.Float32bits(f)
		bytes[i*4] = byte(bits)
		bytes[i*4+1] = byte(bits >> 8)
		bytes[i*4+2] = byte(bits >> 16)
		bytes[i*4+3] = byte(bits >> 24)
	}
	return bytes
}

// ConvertBytesToFloat32Array converts bytes to a float32 array using little-endian encoding
func ConvertBytesToFloat32Array(bytes []byte) []float32 {
	if len(bytes)%4 != 0 {
		return nil
	}
	size := len(bytes) / 4
	floats := make([]float32, size)
	for i := 0; i < size; i++ {
		bits := uint32(bytes[i*4]) |
			uint32(bytes[i*4+1])<<8 |
			uint32(bytes[i*4+2])<<16 |
			uint32(bytes[i*4+3])<<24
		floats[i] = math.Float32frombits(bits)
	}
	return floats
}
