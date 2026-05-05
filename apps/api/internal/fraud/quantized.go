package fraud

const QuantScale int16 = 10000

type QuantizedIndex struct {
	Vectors []int16
	Blocks  []int16
	Labels  []uint8
	IVF     IVFMetadata
	mmap    []byte
}

type IVFMetadata struct {
	Clusters        int
	Centroids       []int16
	CentroidBlocks  []int16
	ListOffsets     []uint32
	BlockOffsets    []uint32
	BBoxMin         []int16
	BBoxMax         []int16
	OrigIDs         []uint32
	NProbe          int
	AmbiguousNProbe int
	Repair          bool
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
	return idx.Search5Quantized(q)
}

func (idx *QuantizedIndex) Search5Quantized(query [Dimensions]int16) int {
	if !idx.hasIVF() {
		return 0
	}
	return idx.searchIVF(query)
}

func NewIVFQuantizedIndex(vectors []int16, labels []uint8, ivf IVFMetadata) *QuantizedIndex {
	if ivf.Clusters > 0 && len(ivf.Centroids) >= ivf.Clusters*Dimensions && len(ivf.CentroidBlocks) == 0 {
		ivf.CentroidBlocks = buildCentroidBlocks(ivf.Centroids, ivf.Clusters)
	}
	idx := &QuantizedIndex{
		Vectors: vectors,
		Labels:  labels,
		IVF:     ivf,
	}
	idx.normalizeIVFDefaults()
	return idx
}

func (idx *QuantizedIndex) SetIVFSearch(nprobe, ambiguousNProbe int, repair bool) {
	if !idx.hasIVF() {
		return
	}
	if nprobe > 0 {
		idx.IVF.NProbe = nprobe
	}
	if ambiguousNProbe > 0 {
		idx.IVF.AmbiguousNProbe = ambiguousNProbe
	}
	idx.IVF.Repair = repair
	idx.normalizeIVFDefaults()
}

func (idx *QuantizedIndex) hasIVF() bool {
	return idx.IVF.Clusters > 0 && len(idx.IVF.ListOffsets) == idx.IVF.Clusters+1 && len(idx.IVF.Centroids) >= idx.IVF.Clusters*Dimensions
}

func (idx *QuantizedIndex) hasIVFBlocks() bool {
	return idx.hasIVF() && len(idx.IVF.BlockOffsets) == idx.IVF.Clusters+1 && len(idx.Blocks) >= int(idx.IVF.BlockOffsets[idx.IVF.Clusters])*ivfBlockStride
}

func (idx *QuantizedIndex) normalizeIVFDefaults() {
	if !idx.hasIVF() {
		return
	}
	if idx.IVF.NProbe <= 0 {
		idx.IVF.NProbe = 8
	}
	if idx.IVF.AmbiguousNProbe <= 0 {
		idx.IVF.AmbiguousNProbe = idx.IVF.NProbe * 5
	}
	if idx.IVF.NProbe > idx.IVF.Clusters {
		idx.IVF.NProbe = idx.IVF.Clusters
	}
	if idx.IVF.AmbiguousNProbe < idx.IVF.NProbe {
		idx.IVF.AmbiguousNProbe = idx.IVF.NProbe
	}
	if idx.IVF.AmbiguousNProbe > idx.IVF.Clusters {
		idx.IVF.AmbiguousNProbe = idx.IVF.Clusters
	}
}

func quantizedDistance(query [Dimensions]int16, ref []int16, cutoff int64) int64 {
	var sum int64
	d := int64(query[5]) - int64(ref[5])
	sum += d * d
	if sum >= cutoff {
		return sum
	}
	d = int64(query[6]) - int64(ref[6])
	sum += d * d
	if sum >= cutoff {
		return sum
	}
	d = int64(query[2]) - int64(ref[2])
	sum += d * d
	if sum >= cutoff {
		return sum
	}
	d = int64(query[0]) - int64(ref[0])
	sum += d * d
	if sum >= cutoff {
		return sum
	}
	d = int64(query[7]) - int64(ref[7])
	sum += d * d
	if sum >= cutoff {
		return sum
	}
	d = int64(query[8]) - int64(ref[8])
	sum += d * d
	if sum >= cutoff {
		return sum
	}
	d = int64(query[12]) - int64(ref[12])
	sum += d * d
	if sum >= cutoff {
		return sum
	}
	d = int64(query[1]) - int64(ref[1])
	sum += d * d
	if sum >= cutoff {
		return sum
	}
	d = int64(query[3]) - int64(ref[3])
	sum += d * d
	if sum >= cutoff {
		return sum
	}
	d = int64(query[4]) - int64(ref[4])
	sum += d * d
	if sum >= cutoff {
		return sum
	}
	d = int64(query[9]) - int64(ref[9])
	sum += d * d
	if sum >= cutoff {
		return sum
	}
	d = int64(query[10]) - int64(ref[10])
	sum += d * d
	if sum >= cutoff {
		return sum
	}
	d = int64(query[11]) - int64(ref[11])
	sum += d * d
	if sum >= cutoff {
		return sum
	}
	d = int64(query[13]) - int64(ref[13])
	sum += d * d
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
