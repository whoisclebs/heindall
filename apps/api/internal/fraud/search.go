package fraud

import "math"

type Searcher interface {
	Search5(query [Dimensions]float32) (frauds int)
}

type ExactSearcher struct {
	references []Reference
}

func NewExactSearcher(references []Reference) *ExactSearcher {
	return &ExactSearcher{references: references}
}

func (s *ExactSearcher) Search5(query [Dimensions]float32) int {
	bestDist := [5]float32{float32(math.MaxFloat32), float32(math.MaxFloat32), float32(math.MaxFloat32), float32(math.MaxFloat32), float32(math.MaxFloat32)}
	bestFraud := [5]bool{}

	for _, ref := range s.references {
		d := squaredDistance(query, ref.Vector)
		if d >= bestDist[4] {
			continue
		}
		fraud := ref.Label == LabelFraud
		for i := 0; i < 5; i++ {
			if d < bestDist[i] {
				for j := 4; j > i; j-- {
					bestDist[j] = bestDist[j-1]
					bestFraud[j] = bestFraud[j-1]
				}
				bestDist[i] = d
				bestFraud[i] = fraud
				break
			}
		}
	}

	frauds := 0
	limit := 5
	if len(s.references) < limit {
		limit = len(s.references)
	}
	for i := 0; i < limit; i++ {
		if bestFraud[i] {
			frauds++
		}
	}
	return frauds
}

func squaredDistance(a, b [Dimensions]float32) float32 {
	var sum float32
	for i := 0; i < Dimensions; i++ {
		d := a[i] - b[i]
		sum += d * d
	}
	return sum
}
