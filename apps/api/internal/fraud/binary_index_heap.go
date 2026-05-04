//go:build !linux

package fraud

func loadBinaryIndex(path string, minCandidates int) (*QuantizedIndex, error) {
	return loadBinaryIndexHeap(path, minCandidates)
}
