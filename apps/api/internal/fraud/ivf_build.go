package fraud

import (
	"fmt"
	"sort"
)

type IVFBuildOptions struct {
	Clusters        int
	NProbe          int
	AmbiguousNProbe int
	Repair          bool
}

func BuildIVFIndex(refs []Reference, opts IVFBuildOptions) (*QuantizedIndex, error) {
	if len(refs) == 0 {
		return nil, fmt.Errorf("reference dataset is empty")
	}
	clusters := opts.Clusters
	if clusters <= 0 {
		clusters = 8192
	}
	if clusters > len(refs) {
		clusters = highestPowerOfTwoLE(len(refs))
	}
	if clusters <= 0 || !isPowerOfTwo(clusters) {
		return nil, fmt.Errorf("IVF clusters must be a positive power of two")
	}

	vectors := make([]int16, len(refs)*Dimensions)
	labels := make([]uint8, len(refs))
	ids := make([]uint32, len(refs))
	for i, ref := range refs {
		q := QuantizeVector(ref.Vector)
		copy(vectors[i*Dimensions:(i+1)*Dimensions], q[:])
		labels[i] = LabelByte(ref.Label)
		ids[i] = uint32(i)
	}

	ranges := make([]ivfBuildRange, clusters)
	balancedSplit(vectors, ids, ranges, 0, len(ids), 0, clusters)
	orderedVectors, orderedLabels, blocks, ivf := materializeIVF(vectors, labels, ids, ranges, opts)
	idx := NewIVFQuantizedIndex(orderedVectors, orderedLabels, ivf)
	idx.Blocks = blocks
	return idx, nil
}

type ivfBuildRange struct {
	start int
	end   int
}

func balancedSplit(vectors []int16, ids []uint32, ranges []ivfBuildRange, start, end, clusterBase, clusterCount int) {
	if clusterCount == 1 {
		ranges[clusterBase] = ivfBuildRange{start: start, end: end}
		return
	}
	dim := maxVarianceDimension(vectors, ids, start, end)
	window := ids[start:end]
	sort.Slice(window, func(i, j int) bool {
		left := window[i]
		right := window[j]
		lv := vectors[int(left)*Dimensions+dim]
		rv := vectors[int(right)*Dimensions+dim]
		if lv == rv {
			return left < right
		}
		return lv < rv
	})
	mid := start + (end-start)/2
	half := clusterCount / 2
	balancedSplit(vectors, ids, ranges, start, mid, clusterBase, half)
	balancedSplit(vectors, ids, ranges, mid, end, clusterBase+half, half)
}

func maxVarianceDimension(vectors []int16, ids []uint32, start, end int) int {
	var sums [Dimensions]int64
	var sumSquares [Dimensions]int64
	for i := start; i < end; i++ {
		base := int(ids[i]) * Dimensions
		for d := 0; d < Dimensions; d++ {
			v := int64(vectors[base+d])
			sums[d] += v
			sumSquares[d] += v * v
		}
	}
	count := float64(end - start)
	bestDim := 0
	bestVariance := -1.0
	for d := 0; d < Dimensions; d++ {
		mean := float64(sums[d]) / count
		variance := float64(sumSquares[d])/count - mean*mean
		if variance > bestVariance {
			bestVariance = variance
			bestDim = d
		}
	}
	return bestDim
}

func materializeIVF(vectors []int16, labels []uint8, ids []uint32, ranges []ivfBuildRange, opts IVFBuildOptions) ([]int16, []uint8, []int16, IVFMetadata) {
	clusters := len(ranges)
	orderedVectors := make([]int16, len(vectors))
	orderedLabels := make([]uint8, len(labels))
	centroids := make([]int16, clusters*Dimensions)
	listOffsets := make([]uint32, clusters+1)
	blockOffsets := make([]uint32, clusters+1)
	bboxMin := make([]int16, clusters*Dimensions)
	bboxMax := make([]int16, clusters*Dimensions)
	origIDs := make([]uint32, len(labels))
	pos := 0
	blockPos := 0
	for c, r := range ranges {
		listOffsets[c] = uint32(pos)
		blockOffsets[c] = uint32(blockPos)
		computeClusterStats(vectors, ids, r, centroids[c*Dimensions:(c+1)*Dimensions], bboxMin[c*Dimensions:(c+1)*Dimensions], bboxMax[c*Dimensions:(c+1)*Dimensions])
		clusterIDs := sortClusterByCentroidDistance(vectors, ids[r.start:r.end], centroids[c*Dimensions:(c+1)*Dimensions])
		for _, origID := range clusterIDs {
			copy(orderedVectors[pos*Dimensions:(pos+1)*Dimensions], vectors[int(origID)*Dimensions:int(origID)*Dimensions+Dimensions])
			orderedLabels[pos] = labels[origID]
			origIDs[pos] = origID
			pos++
		}
		listOffsets[c+1] = uint32(pos)
		blockPos += blocksForRows(len(clusterIDs))
		blockOffsets[c+1] = uint32(blockPos)
	}
	blocks := buildIVFBlocks(orderedVectors, listOffsets, blockOffsets)
	return orderedVectors, orderedLabels, blocks, IVFMetadata{
		Clusters:        clusters,
		Centroids:       centroids,
		ListOffsets:     listOffsets,
		BlockOffsets:    blockOffsets,
		BBoxMin:         bboxMin,
		BBoxMax:         bboxMax,
		OrigIDs:         origIDs,
		NProbe:          opts.NProbe,
		AmbiguousNProbe: opts.AmbiguousNProbe,
		Repair:          opts.Repair,
	}
}

func blocksForRows(rows int) int {
	if rows <= 0 {
		return 0
	}
	return (rows + ivfBlockSize - 1) / ivfBlockSize
}

func buildIVFBlocks(vectors []int16, listOffsets, blockOffsets []uint32) []int16 {
	blockCount := int(blockOffsets[len(blockOffsets)-1])
	blocks := make([]int16, blockCount*ivfBlockStride)
	for c := 0; c+1 < len(listOffsets); c++ {
		rowStart := int(listOffsets[c])
		rowEnd := int(listOffsets[c+1])
		blockStart := int(blockOffsets[c])
		for row := rowStart; row < rowEnd; row++ {
			rel := row - rowStart
			block := blockStart + rel/ivfBlockSize
			lane := rel % ivfBlockSize
			blockBase := block * ivfBlockStride
			vectorBase := row * Dimensions
			for d := 0; d < Dimensions; d++ {
				blocks[blockBase+d*ivfBlockSize+lane] = vectors[vectorBase+d]
			}
		}
	}
	return blocks
}

func buildCentroidBlocks(centroids []int16, clusters int) []int16 {
	blockCount := blocksForRows(clusters)
	blocks := make([]int16, blockCount*ivfBlockStride)
	for c := 0; c < clusters; c++ {
		block := c / ivfBlockSize
		lane := c % ivfBlockSize
		blockBase := block * ivfBlockStride
		centroidBase := c * Dimensions
		for d := 0; d < Dimensions; d++ {
			blocks[blockBase+d*ivfBlockSize+lane] = centroids[centroidBase+d]
		}
	}
	return blocks
}

func computeClusterStats(vectors []int16, ids []uint32, r ivfBuildRange, centroid, bboxMin, bboxMax []int16) {
	for d := 0; d < Dimensions; d++ {
		bboxMin[d] = 32767
		bboxMax[d] = -32768
	}
	var sums [Dimensions]int64
	for i := r.start; i < r.end; i++ {
		base := int(ids[i]) * Dimensions
		for d := 0; d < Dimensions; d++ {
			v := vectors[base+d]
			sums[d] += int64(v)
			if v < bboxMin[d] {
				bboxMin[d] = v
			}
			if v > bboxMax[d] {
				bboxMax[d] = v
			}
		}
	}
	count := int64(r.end - r.start)
	for d := 0; d < Dimensions; d++ {
		centroid[d] = roundDivInt64ToInt16(sums[d], count)
	}
}

type ivfIDDistance struct {
	id   uint32
	dist int64
}

func sortClusterByCentroidDistance(vectors []int16, ids []uint32, centroid []int16) []uint32 {
	items := make([]ivfIDDistance, len(ids))
	for i, id := range ids {
		items[i] = ivfIDDistance{id: id, dist: vectorToCentroidDistance(vectors, id, centroid)}
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].dist == items[j].dist {
			return items[i].id < items[j].id
		}
		return items[i].dist < items[j].dist
	})
	out := make([]uint32, len(ids))
	for i, item := range items {
		out[i] = item.id
	}
	return out
}

func vectorToCentroidDistance(vectors []int16, id uint32, centroid []int16) int64 {
	base := int(id) * Dimensions
	var sum int64
	for d := 0; d < Dimensions; d++ {
		delta := int64(vectors[base+d]) - int64(centroid[d])
		sum += delta * delta
	}
	return sum
}

func roundDivInt64ToInt16(sum, count int64) int16 {
	if count <= 0 {
		return 0
	}
	if sum >= 0 {
		return int16((sum + count/2) / count)
	}
	return int16((sum - count/2) / count)
}

func isPowerOfTwo(v int) bool {
	return v > 0 && v&(v-1) == 0
}

func highestPowerOfTwoLE(v int) int {
	if v <= 0 {
		return 0
	}
	p := 1
	for p <= v/2 {
		p *= 2
	}
	return p
}
