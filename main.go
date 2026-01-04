package main

import (
	"math/rand/v2"

	"github.com/samber/lo"
	"github.com/tsingjyujing/vestigo/cmd"
)

// RandomVec random vector for testing
func RandomVec(dimension int) []float32 {
	return lo.Map(
		lo.Range(dimension),
		func(_ int, _ int) float32 {
			return rand.Float32()
		},
	)
}

func main() {
	cmd.Execute()
}
