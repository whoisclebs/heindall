//go:build linux

package fraud

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

func loadBinaryIndex(path string) (*QuantizedIndex, error) {
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
	return loadIVFBinaryIndexMmap(data, header)
}

func loadIVFBinaryIndexMmap(data []byte, header binaryHeader) (*QuantizedIndex, error) {
	count := int(header.Count)
	pos := binaryHeaderSize()
	if len(data) < pos+ivfBinaryHeaderSize() {
		syscall.Munmap(data)
		return nil, fmt.Errorf("truncated IVF binary index")
	}
	ivfHeader := (*ivfBinaryHeader)(unsafe.Pointer(&data[pos]))
	pos += ivfBinaryHeaderSize()
	clusters := int(ivfHeader.Clusters)
	if clusters <= 0 {
		syscall.Munmap(data)
		return nil, fmt.Errorf("invalid IVF cluster count")
	}

	centroidsCount := clusters * Dimensions
	centroidsBytes := centroidsCount * 2
	listOffsetsCount := clusters + 1
	listOffsetsBytes := listOffsetsCount * 4
	bboxCount := clusters * Dimensions
	bboxBytes := bboxCount * 2
	origIDsBytes := count * 4
	vectorCount := count * Dimensions
	vectorBytes := vectorCount * 2
	need := pos + centroidsBytes + listOffsetsBytes + bboxBytes + bboxBytes + origIDsBytes + vectorBytes + count
	if len(data) < need {
		syscall.Munmap(data)
		return nil, fmt.Errorf("truncated IVF binary index")
	}

	centroids := unsafe.Slice((*int16)(unsafe.Pointer(&data[pos])), centroidsCount)
	pos += centroidsBytes
	listOffsets := unsafe.Slice((*uint32)(unsafe.Pointer(&data[pos])), listOffsetsCount)
	pos += listOffsetsBytes
	bboxMin := unsafe.Slice((*int16)(unsafe.Pointer(&data[pos])), bboxCount)
	pos += bboxBytes
	bboxMax := unsafe.Slice((*int16)(unsafe.Pointer(&data[pos])), bboxCount)
	pos += bboxBytes
	origIDs := unsafe.Slice((*uint32)(unsafe.Pointer(&data[pos])), count)
	pos += origIDsBytes
	vectors := unsafe.Slice((*int16)(unsafe.Pointer(&data[pos])), vectorCount)
	pos += vectorBytes
	labels := data[pos : pos+count]

	idx := NewIVFQuantizedIndex(vectors, labels, IVFMetadata{
		Clusters:        clusters,
		Centroids:       centroids,
		ListOffsets:     listOffsets,
		BBoxMin:         bboxMin,
		BBoxMax:         bboxMax,
		OrigIDs:         origIDs,
		NProbe:          int(ivfHeader.NProbe),
		AmbiguousNProbe: int(ivfHeader.AmbiguousNProbe),
		Repair:          ivfHeader.Flags&1 == 1,
	})
	idx.mmap = data
	return idx, nil
}
