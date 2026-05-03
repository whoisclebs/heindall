package fraud

const QuantScale int16 = 10000

type QuantizedIndex struct {
	Vectors       []int16
	Labels        []uint8
	Buckets       map[uint64][]uint32
	MinCandidates int
}

func NewQuantizedIndex(vectors []int16, labels []uint8, minCandidates int) *QuantizedIndex {
	if minCandidates <= 0 {
		minCandidates = 8192
	}
	idx := &QuantizedIndex{
		Vectors:       vectors,
		Labels:        labels,
		Buckets:       make(map[uint64][]uint32, len(labels)/64),
		MinCandidates: minCandidates,
	}
	idx.buildBuckets()
	return idx
}

func QuantizeVector(vector [Dimensions]float32) [Dimensions]int16 {
	var out [Dimensions]int16
	for i, value := range vector {
		if value <= -1 {
			out[i] = -QuantScale
			continue
		}
		if value < 0 {
			value = 0
		}
		if value > 1 {
			value = 1
		}
		out[i] = int16(value*float32(QuantScale) + 0.5)
	}
	return out
}

func LabelByte(label string) uint8 {
	if label == LabelFraud {
		return 1
	}
	return 0
}

func (idx *QuantizedIndex) Search5(query [Dimensions]float32) int {
	q := QuantizeVector(query)
	frauds, visited := idx.searchBuckets(q)
	if visited == 0 {
		return idx.searchAll(q)
	}
	return frauds
}

func (idx *QuantizedIndex) searchBuckets(query [Dimensions]int16) (frauds int, visited int) {
	bestDist := [5]int64{1<<62 - 1, 1<<62 - 1, 1<<62 - 1, 1<<62 - 1, 1<<62 - 1}
	bestFraud := [5]bool{}
	b0 := bucket(query[0])
	b2 := bucket(query[2])
	b7 := bucket(query[7])
	b12 := bucket(query[12])

	for radius := int16(0); radius <= 2 && visited < idx.MinCandidates; radius++ {
		for d0 := -radius; d0 <= radius; d0++ {
			for d2 := -radius; d2 <= radius; d2++ {
				for d7 := -radius; d7 <= radius; d7++ {
					for d12 := -radius; d12 <= radius; d12++ {
						if radius > 0 && abs16(d0) < radius && abs16(d2) < radius && abs16(d7) < radius && abs16(d12) < radius {
							continue
						}
						key := packedBucketKey(b0+d0, b2+d2, b7+d7, b12+d12)
						for _, pos := range idx.Buckets[key] {
							idx.offer(query, pos, &bestDist, &bestFraud)
							visited++
						}
					}
				}
			}
		}
	}
	return countFrauds(bestFraud), visited
}

func (idx *QuantizedIndex) buildBuckets() {
	count := len(idx.Labels)
	for i := 0; i < count; i++ {
		vec := idx.Vectors[i*Dimensions : (i+1)*Dimensions]
		key := bucketKey(vec[0], vec[2], vec[7], vec[12])
		idx.Buckets[key] = append(idx.Buckets[key], uint32(i))
	}
}

func (idx *QuantizedIndex) searchAll(query [Dimensions]int16) int {
	bestDist := [5]int64{1<<62 - 1, 1<<62 - 1, 1<<62 - 1, 1<<62 - 1, 1<<62 - 1}
	bestFraud := [5]bool{}
	for i := range idx.Labels {
		idx.offer(query, uint32(i), &bestDist, &bestFraud)
	}
	return countFrauds(bestFraud)
}

func (idx *QuantizedIndex) offer(query [Dimensions]int16, pos uint32, bestDist *[5]int64, bestFraud *[5]bool) {
	start := int(pos) * Dimensions
	d := quantizedDistance(query, idx.Vectors[start:start+Dimensions], bestDist[4])
	if d >= bestDist[4] {
		return
	}
	fraud := idx.Labels[pos] == 1
	for i := 0; i < 5; i++ {
		if d < bestDist[i] {
			for j := 4; j > i; j-- {
				bestDist[j] = bestDist[j-1]
				bestFraud[j] = bestFraud[j-1]
			}
			bestDist[i] = d
			bestFraud[i] = fraud
			return
		}
	}
}

func quantizedDistance(query [Dimensions]int16, ref []int16, cutoff int64) int64 {
	order := [...]int{5, 6, 2, 0, 7, 8, 12, 1, 3, 4, 9, 10, 11, 13}
	var sum int64
	for _, dim := range order {
		d := int64(query[dim]) - int64(ref[dim])
		sum += d * d
		if sum >= cutoff {
			return sum
		}
	}
	return sum
}

func countFrauds(bestFraud [5]bool) int {
	frauds := 0
	for _, fraud := range bestFraud {
		if fraud {
			frauds++
		}
	}
	return frauds
}

func bucketKey(v0, v2, v7, v12 int16) uint64 {
	return packedBucketKey(bucket(v0), bucket(v2), bucket(v7), bucket(v12))
}

func packedBucketKey(b0, b2, b7, b12 int16) uint64 {
	return uint64(clampBucket(b0))<<24 | uint64(clampBucket(b2))<<16 | uint64(clampBucket(b7))<<8 | uint64(clampBucket(b12))
}

func bucket(v int16) int16 {
	if v < 0 {
		return 0
	}
	return v / 625 // 16 buckets over [0, 10000]
}

func clampBucket(v int16) uint8 {
	if v < 0 {
		return 0
	}
	if v > 15 {
		return 15
	}
	return uint8(v)
}

func abs16(v int16) int16 {
	if v < 0 {
		return -v
	}
	return v
}
