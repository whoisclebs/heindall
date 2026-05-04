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

func WriteBinaryIndex(path string, refs []Reference) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	header := binaryHeader{Magic: binaryMagic, Version: 1, Dimensions: Dimensions, Scale: int32(QuantScale), Count: uint32(len(refs))}
	if err := binary.Write(f, binary.LittleEndian, header); err != nil {
		return err
	}

	for _, ref := range refs {
		q := QuantizeVector(ref.Vector)
		if err := binary.Write(f, binary.LittleEndian, q); err != nil {
			return err
		}
	}
	for _, ref := range refs {
		label := LabelByte(ref.Label)
		if err := binary.Write(f, binary.LittleEndian, label); err != nil {
			return err
		}
	}
	return nil
}

func LoadBinaryIndex(path string, minCandidates int) (*QuantizedIndex, error) {
	return loadBinaryIndex(path, minCandidates)
}

func readBinaryHeader(f *os.File) (binaryHeader, error) {
	var header binaryHeader
	if err := binary.Read(f, binary.LittleEndian, &header); err != nil {
		return binaryHeader{}, err
	}
	if header.Magic != binaryMagic || header.Version != 1 || header.Dimensions != Dimensions || header.Scale != int32(QuantScale) {
		return binaryHeader{}, fmt.Errorf("unsupported index format")
	}
	return header, nil
}

func binaryHeaderSize() int {
	return binary.Size(binaryHeader{})
}

func loadBinaryIndexHeap(path string, minCandidates int) (*QuantizedIndex, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	header, err := readBinaryHeader(f)
	if err != nil {
		return nil, err
	}

	count := int(header.Count)
	vectors := make([]int16, count*Dimensions)
	if err := binary.Read(f, binary.LittleEndian, vectors); err != nil {
		return nil, err
	}
	labels := make([]uint8, count)
	if _, err := io.ReadFull(f, labels); err != nil {
		return nil, err
	}
	return NewQuantizedIndex(vectors, labels, minCandidates), nil
}
