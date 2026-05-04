//go:build !linux

package fraud

func loadBinaryIndex(path string) (*QuantizedIndex, error) {
	return loadBinaryIndexHeap(path)
}
