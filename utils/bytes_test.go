package utils

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConvertFloat32ArrayToBytes(t *testing.T) {
	tests := []struct {
		name     string
		input    []float32
		expected []byte
	}{
		{
			name:     "Empty array",
			input:    []float32{},
			expected: []byte{},
		},
		{
			name:     "Single zero",
			input:    []float32{0.0},
			expected: []byte{0x00, 0x00, 0x00, 0x00},
		},
		{
			name:     "Single positive number",
			input:    []float32{1.0},
			expected: []byte{0x00, 0x00, 0x80, 0x3f}, // IEEE 754 representation of 1.0
		},
		{
			name:     "Single negative number",
			input:    []float32{-1.0},
			expected: []byte{0x00, 0x00, 0x80, 0xbf}, // IEEE 754 representation of -1.0
		},
		{
			name:  "Multiple numbers",
			input: []float32{1.0, 2.0, 3.0},
			expected: []byte{
				0x00, 0x00, 0x80, 0x3f, // 1.0
				0x00, 0x00, 0x00, 0x40, // 2.0
				0x00, 0x00, 0x40, 0x40, // 3.0
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConvertFloat32ArrayToBytes(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConvertBytesToFloat32Array(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected []float32
	}{
		{
			name:     "Empty array",
			input:    []byte{},
			expected: []float32{},
		},
		{
			name:     "Single zero",
			input:    []byte{0x00, 0x00, 0x00, 0x00},
			expected: []float32{0.0},
		},
		{
			name:     "Single positive number",
			input:    []byte{0x00, 0x00, 0x80, 0x3f}, // IEEE 754 representation of 1.0
			expected: []float32{1.0},
		},
		{
			name:     "Single negative number",
			input:    []byte{0x00, 0x00, 0x80, 0xbf}, // IEEE 754 representation of -1.0
			expected: []float32{-1.0},
		},
		{
			name: "Multiple numbers",
			input: []byte{
				0x00, 0x00, 0x80, 0x3f, // 1.0
				0x00, 0x00, 0x00, 0x40, // 2.0
				0x00, 0x00, 0x40, 0x40, // 3.0
			},
			expected: []float32{1.0, 2.0, 3.0},
		},
		{
			name:     "Invalid length (not multiple of 4)",
			input:    []byte{0x00, 0x00, 0x80},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConvertBytesToFloat32Array(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRoundTripConversion(t *testing.T) {
	tests := []struct {
		name  string
		input []float32
	}{
		{
			name:  "Empty array",
			input: []float32{},
		},
		{
			name:  "Single element",
			input: []float32{3.14159},
		},
		{
			name:  "Multiple positive numbers",
			input: []float32{1.0, 2.0, 3.0, 4.0, 5.0},
		},
		{
			name:  "Mixed positive and negative",
			input: []float32{-10.5, 0.0, 10.5, -3.14, 2.71},
		},
		{
			name:  "Special values",
			input: []float32{0.0, -0.0, math.MaxFloat32, -math.MaxFloat32},
		},
		{
			name:  "Small decimals",
			input: []float32{0.001, 0.0001, 0.00001},
		},
		{
			name:  "Large numbers",
			input: []float32{1e10, 1e20, 1e30},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Convert to bytes and back
			bytes := ConvertFloat32ArrayToBytes(tt.input)
			result := ConvertBytesToFloat32Array(bytes)

			// Check length
			assert.Equal(t, len(tt.input), len(result))

			// Check each element
			for i := range tt.input {
				// Use InDelta for floating point comparison to handle precision issues
				assert.InDelta(t, tt.input[i], result[i], 1e-6,
					"Mismatch at index %d: expected %v, got %v", i, tt.input[i], result[i])
			}
		})
	}
}

func TestSpecialFloatValues(t *testing.T) {
	tests := []struct {
		name  string
		value float32
	}{
		{name: "Positive infinity", value: float32(math.Inf(1))},
		{name: "Negative infinity", value: float32(math.Inf(-1))},
		{name: "NaN", value: float32(math.NaN())},
		{name: "Zero", value: 0.0},
		{name: "Negative zero", value: float32(math.Copysign(0, -1))},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := []float32{tt.value}
			bytes := ConvertFloat32ArrayToBytes(input)
			result := ConvertBytesToFloat32Array(bytes)

			assert.Equal(t, len(input), len(result))

			// Special handling for NaN since NaN != NaN
			if math.IsNaN(float64(tt.value)) {
				assert.True(t, math.IsNaN(float64(result[0])), "Expected NaN")
			} else {
				assert.Equal(t, tt.value, result[0])
			}
		})
	}
}

func TestBytesLength(t *testing.T) {
	tests := []struct {
		name          string
		inputFloats   int
		expectedBytes int
	}{
		{name: "0 floats", inputFloats: 0, expectedBytes: 0},
		{name: "1 float", inputFloats: 1, expectedBytes: 4},
		{name: "10 floats", inputFloats: 10, expectedBytes: 40},
		{name: "100 floats", inputFloats: 100, expectedBytes: 400},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := make([]float32, tt.inputFloats)
			bytes := ConvertFloat32ArrayToBytes(input)
			assert.Equal(t, tt.expectedBytes, len(bytes))
		})
	}
}
