package fraud

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
)

var binaryMagic = [8]byte{'H', 'N', 'D', 'X', '2', '0', '2', '6'}

type binaryHeader struct {
	Magic      [8]byte
	Version    uint32
	Dimensions uint32
	Scale      int32
	Count      uint32
}

type ivfBinaryHeader struct {
	Clusters        uint32
	NProbe          uint32
	AmbiguousNProbe uint32
	Flags           uint32
}

func WriteIVFBinaryIndex(path string, refs []Reference, opts IVFBuildOptions) error {
	idx, err := BuildIVFIndex(refs, opts)
	if err != nil {
		return err
	}
	return writeIVFQuantizedIndex(path, idx)
}

func writeIVFQuantizedIndex(path string, idx *QuantizedIndex) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	header := binaryHeader{Magic: binaryMagic, Version: 2, Dimensions: Dimensions, Scale: int32(QuantScale), Count: uint32(len(idx.Labels))}
	if err := binary.Write(f, binary.LittleEndian, header); err != nil {
		return err
	}
	flags := uint32(0)
	if idx.IVF.Repair {
		flags |= 1
	}
	ivfHeader := ivfBinaryHeader{
		Clusters:        uint32(idx.IVF.Clusters),
		NProbe:          uint32(idx.IVF.NProbe),
		AmbiguousNProbe: uint32(idx.IVF.AmbiguousNProbe),
		Flags:           flags,
	}
	if err := binary.Write(f, binary.LittleEndian, ivfHeader); err != nil {
		return err
	}
	if err := binary.Write(f, binary.LittleEndian, idx.IVF.Centroids); err != nil {
		return err
	}
	if err := binary.Write(f, binary.LittleEndian, idx.IVF.ListOffsets); err != nil {
		return err
	}
	if err := binary.Write(f, binary.LittleEndian, idx.IVF.BBoxMin); err != nil {
		return err
	}
	if err := binary.Write(f, binary.LittleEndian, idx.IVF.BBoxMax); err != nil {
		return err
	}
	if err := binary.Write(f, binary.LittleEndian, idx.IVF.OrigIDs); err != nil {
		return err
	}
	if err := binary.Write(f, binary.LittleEndian, idx.Vectors); err != nil {
		return err
	}
	if _, err := f.Write(idx.Labels); err != nil {
		return err
	}
	return nil
}

func LoadBinaryIndex(path string) (*QuantizedIndex, error) {
	return loadBinaryIndex(path)
}

func readBinaryHeader(f *os.File) (binaryHeader, error) {
	var header binaryHeader
	if err := binary.Read(f, binary.LittleEndian, &header); err != nil {
		return binaryHeader{}, err
	}
	if header.Magic != binaryMagic || header.Dimensions != Dimensions || header.Scale != int32(QuantScale) {
		return binaryHeader{}, fmt.Errorf("unsupported index format")
	}
	if header.Version != 2 {
		return binaryHeader{}, fmt.Errorf("unsupported index version %d", header.Version)
	}
	return header, nil
}

func binaryHeaderSize() int {
	return binary.Size(binaryHeader{})
}

func ivfBinaryHeaderSize() int {
	return binary.Size(ivfBinaryHeader{})
}

func loadBinaryIndexHeap(path string) (*QuantizedIndex, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	header, err := readBinaryHeader(f)
	if err != nil {
		return nil, err
	}
	return loadIVFBinaryIndexHeap(f, header)
}

func loadIVFBinaryIndexHeap(f *os.File, header binaryHeader) (*QuantizedIndex, error) {
	var ih ivfBinaryHeader
	if err := binary.Read(f, binary.LittleEndian, &ih); err != nil {
		return nil, err
	}
	count := int(header.Count)
	clusters := int(ih.Clusters)
	if clusters <= 0 {
		return nil, fmt.Errorf("invalid IVF cluster count")
	}
	centroids := make([]int16, clusters*Dimensions)
	if err := binary.Read(f, binary.LittleEndian, centroids); err != nil {
		return nil, err
	}
	listOffsets := make([]uint32, clusters+1)
	if err := binary.Read(f, binary.LittleEndian, listOffsets); err != nil {
		return nil, err
	}
	bboxMin := make([]int16, clusters*Dimensions)
	if err := binary.Read(f, binary.LittleEndian, bboxMin); err != nil {
		return nil, err
	}
	bboxMax := make([]int16, clusters*Dimensions)
	if err := binary.Read(f, binary.LittleEndian, bboxMax); err != nil {
		return nil, err
	}
	origIDs := make([]uint32, count)
	if err := binary.Read(f, binary.LittleEndian, origIDs); err != nil {
		return nil, err
	}
	vectors := make([]int16, count*Dimensions)
	if err := binary.Read(f, binary.LittleEndian, vectors); err != nil {
		return nil, err
	}
	labels := make([]uint8, count)
	if _, err := io.ReadFull(f, labels); err != nil {
		return nil, err
	}
	idx := NewIVFQuantizedIndex(vectors, labels, IVFMetadata{
		Clusters:        clusters,
		Centroids:       centroids,
		ListOffsets:     listOffsets,
		BBoxMin:         bboxMin,
		BBoxMax:         bboxMax,
		OrigIDs:         origIDs,
		NProbe:          int(ih.NProbe),
		AmbiguousNProbe: int(ih.AmbiguousNProbe),
		Repair:          ih.Flags&1 == 1,
	})
	return idx, nil
}
