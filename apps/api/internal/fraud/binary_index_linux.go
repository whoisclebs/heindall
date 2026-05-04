//go:build linux

package fraud

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

func loadBinaryIndex(path string, minCandidates int) (*QuantizedIndex, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	header, err := readBinaryHeader(f)
	if err != nil {
		return nil, err
	}

	stat, err := f.Stat()
	if err != nil {
		return nil, err
	}
	data, err := syscall.Mmap(int(f.Fd()), 0, int(stat.Size()), syscall.PROT_READ, syscall.MAP_PRIVATE)
	if err != nil {
		return nil, err
	}

	count := int(header.Count)
	headerSize := binaryHeaderSize()
	vectorBytes := count * Dimensions * 2
	labelOffset := headerSize + vectorBytes
	if len(data) < labelOffset+count {
		syscall.Munmap(data)
		return nil, fmt.Errorf("truncated binary index")
	}

	vectors := unsafe.Slice((*int16)(unsafe.Pointer(&data[headerSize])), count*Dimensions)
	labels := data[labelOffset : labelOffset+count]
	idx := NewQuantizedIndex(vectors, labels, minCandidates)
	idx.mmap = data
	return idx, nil
}
