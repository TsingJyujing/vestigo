package main

import (
	"fmt"
	"log"

	"math/rand/v2"

	"github.com/samber/lo"
	"github.com/tsingjyujing/vestigo/cmd"
	"github.com/wizenheimer/comet"
)

func mainBak() {
	cmd.Execute()
}

func randomVec(dimension int) []float32 {
	return lo.Map(
		lo.Range(dimension),
		func(_ int, _ int) float32 {
			return rand.Float32()
		},
	)
}

func main() {
	const dimension = 384
	index, err := comet.NewFlatIndex(dimension, comet.Cosine)
	if err != nil {
		log.Fatal(err)

	}
	// Add random vectors
	vec1 := randomVec(dimension)

	// ... populate vec1 with your embedding ...
	node := comet.NewVectorNode(vec1)
	err = index.Add(*node)
	if err != nil {
		log.Fatal(err)
	}

	// Search for similar vectors
	query := randomVec(dimension)
	// ... populate query vector ...
	results, err := index.NewSearch().
		WithQuery(query).
		WithK(10).
		Execute()

	if err != nil {
		log.Fatal(err)
	}

	// Process results
	for i, result := range results {
		fmt.Printf("%d. ID=%d, Score=%.4f\n", i+1, result.GetId(), result.GetScore())
	}

}
