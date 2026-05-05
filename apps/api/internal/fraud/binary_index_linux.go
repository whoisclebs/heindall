//go:build linux

package fraud

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

var mmapPreloadSink byte

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
	preloadMmap(data)
	return loadIVFBinaryIndexMmap(data, header)
}

func preloadMmap(data []byte) {
	if len(data) == 0 {
		return
	}
	_ = syscall.Madvise(data, syscall.MADV_WILLNEED)
	var sum byte
	for offset := 0; offset < len(data); offset += 4096 {
		sum ^= data[offset]
	}
	sum ^= data[len(data)-1]
	mmapPreloadSink ^= sum
	_ = syscall.Madvise(data, syscall.MADV_RANDOM)
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
	blockOffsetsBytes := 0
	if header.Version >= 3 {
		blockOffsetsBytes = listOffsetsCount * 4
	}
	labelPadding := 0
	if header.Version >= 3 && count%2 != 0 {
		labelPadding = 1
	}
	bboxCount := clusters * Dimensions
	bboxBytes := bboxCount * 2
	origIDsBytes := count * 4
	vectorBytes := 0
	if header.Version == 2 {
		vectorBytes = count * Dimensions * 2
	}
	need := pos + centroidsBytes + listOffsetsBytes + blockOffsetsBytes + bboxBytes + bboxBytes + origIDsBytes + vectorBytes + count + labelPadding
	if len(data) < need {
		syscall.Munmap(data)
		return nil, fmt.Errorf("truncated IVF binary index")
	}

	centroids := unsafe.Slice((*int16)(unsafe.Pointer(&data[pos])), centroidsCount)
	pos += centroidsBytes
	listOffsets := unsafe.Slice((*uint32)(unsafe.Pointer(&data[pos])), listOffsetsCount)
	pos += listOffsetsBytes
	var blockOffsets []uint32
	if header.Version >= 3 {
		blockOffsets = unsafe.Slice((*uint32)(unsafe.Pointer(&data[pos])), listOffsetsCount)
		pos += blockOffsetsBytes
	}
	if err := validateIVFOffsets(count, clusters, listOffsets, blockOffsets); err != nil {
		syscall.Munmap(data)
		return nil, err
	}
	bboxMin := unsafe.Slice((*int16)(unsafe.Pointer(&data[pos])), bboxCount)
	pos += bboxBytes
	bboxMax := unsafe.Slice((*int16)(unsafe.Pointer(&data[pos])), bboxCount)
	pos += bboxBytes
	origIDs := unsafe.Slice((*uint32)(unsafe.Pointer(&data[pos])), count)
	pos += origIDsBytes
	var vectors []int16
	var blocks []int16
	if header.Version == 2 {
		vectors = unsafe.Slice((*int16)(unsafe.Pointer(&data[pos])), count*Dimensions)
		pos += vectorBytes
	}
	labels := data[pos : pos+count]
	pos += count + labelPadding
	if header.Version >= 3 {
		blockCount := int(blockOffsets[len(blockOffsets)-1])
		blockBytes := blockCount * ivfBlockStride * 2
		if len(data) < pos+blockBytes {
			syscall.Munmap(data)
			return nil, fmt.Errorf("truncated IVF binary index")
		}
		blocks = unsafe.Slice((*int16)(unsafe.Pointer(&data[pos])), blockCount*ivfBlockStride)
	}

	idx := NewIVFQuantizedIndex(vectors, labels, IVFMetadata{
		Clusters:        clusters,
		Centroids:       centroids,
		ListOffsets:     listOffsets,
		BlockOffsets:    blockOffsets,
		BBoxMin:         bboxMin,
		BBoxMax:         bboxMax,
		OrigIDs:         origIDs,
		NProbe:          int(ivfHeader.NProbe),
		AmbiguousNProbe: int(ivfHeader.AmbiguousNProbe),
		Repair:          ivfHeader.Flags&1 == 1,
	})
	idx.Blocks = blocks
	idx.mmap = data
	return idx, nil
}
