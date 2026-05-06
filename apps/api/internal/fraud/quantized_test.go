package fraud

import (
	"testing"
	"unsafe"
)

func TestIVFQuantizedIndexRoundTrip(t *testing.T) {
	path := t.TempDir() + "/index.ivf.bin"
	refs := []Reference{
		{Vector: withFirstDim(0.01), Label: LabelFraud},
		{Vector: withFirstDim(0.02), Label: LabelLegit},
		{Vector: withFirstDim(0.03), Label: LabelFraud},
		{Vector: withFirstDim(0.04), Label: LabelLegit},
		{Vector: withFirstDim(0.05), Label: LabelFraud},
		{Vector: withFirstDim(0.90), Label: LabelLegit},
		{Vector: withFirstDim(0.95), Label: LabelLegit},
		{Vector: withFirstDim(1.00), Label: LabelFraud},
	}

	if err := WriteIVFBinaryIndex(path, refs, IVFBuildOptions{Clusters: 2, NProbe: 1, AmbiguousNProbe: 2, Repair: true}); err != nil {
		t.Fatal(err)
	}
	idx, err := LoadBinaryIndex(path)
	if err != nil {
		t.Fatal(err)
	}
	if !idx.hasIVF() {
		t.Fatal("loaded index is not IVF")
	}
	if len(idx.IVF.CentroidBlocks) != blocksForRows(idx.IVF.Clusters)*ivfBlockStride {
		t.Fatal("loaded index did not materialize centroid blocks")
	}
	if got := idx.Search5([Dimensions]float32{}); got != 3 {
		t.Fatalf("frauds among nearest = %d, want 3", got)
	}
}

func TestIVFSearchMatchesExactWhenAllListsAreProbed(t *testing.T) {
	refs := []Reference{
		{Vector: withFirstDim(0.01), Label: LabelFraud},
		{Vector: withFirstDim(0.02), Label: LabelLegit},
		{Vector: withFirstDim(0.03), Label: LabelFraud},
		{Vector: withFirstDim(0.04), Label: LabelLegit},
		{Vector: withFirstDim(0.05), Label: LabelFraud},
		{Vector: withFirstDim(0.90), Label: LabelLegit},
		{Vector: withFirstDim(0.95), Label: LabelLegit},
		{Vector: withFirstDim(1.00), Label: LabelFraud},
	}
	idx, err := BuildIVFIndex(refs, IVFBuildOptions{Clusters: 4, NProbe: 4, AmbiguousNProbe: 4, Repair: true})
	if err != nil {
		t.Fatal(err)
	}
	want := NewExactSearcher(refs).Search5([Dimensions]float32{})
	if got := idx.Search5([Dimensions]float32{}); got != want {
		t.Fatalf("frauds among nearest = %d, want %d", got, want)
	}
}

func TestIVFTransposedBlockRoundTripHandlesPartialBlock(t *testing.T) {
	path := t.TempDir() + "/index-v3-partial.ivf.bin"
	refs := []Reference{
		{Vector: withFirstDim(0.01), Label: LabelFraud},
		{Vector: withFirstDim(0.02), Label: LabelLegit},
		{Vector: withFirstDim(0.03), Label: LabelFraud},
		{Vector: withFirstDim(0.04), Label: LabelLegit},
		{Vector: withFirstDim(0.05), Label: LabelFraud},
		{Vector: withFirstDim(0.06), Label: LabelLegit},
		{Vector: withFirstDim(0.07), Label: LabelFraud},
		{Vector: withFirstDim(0.08), Label: LabelLegit},
		{Vector: withFirstDim(0.09), Label: LabelFraud},
	}

	if err := WriteIVFBinaryIndex(path, refs, IVFBuildOptions{Clusters: 1, NProbe: 1, AmbiguousNProbe: 1, Repair: true}); err != nil {
		t.Fatal(err)
	}
	idx, err := LoadBinaryIndex(path)
	if err != nil {
		t.Fatal(err)
	}
	if !idx.hasIVFBlocks() {
		t.Fatal("loaded v3 index does not expose transposed IVF blocks")
	}
	want := NewExactSearcher(refs).Search5([Dimensions]float32{})
	if got := idx.Search5([Dimensions]float32{}); got != want {
		t.Fatalf("frauds among nearest = %d, want %d", got, want)
	}
}

func TestAVX2BlockDistancesMatchScalar(t *testing.T) {
	if !useIVFAVX2 {
		t.Skip("AVX2 unavailable")
	}
	idx, err := BuildIVFIndex([]Reference{
		{Vector: withFirstDim(0.01), Label: LabelFraud},
		{Vector: withFirstDim(0.02), Label: LabelLegit},
		{Vector: withFirstDim(0.03), Label: LabelFraud},
		{Vector: withFirstDim(0.04), Label: LabelLegit},
		{Vector: withFirstDim(0.05), Label: LabelFraud},
		{Vector: withFirstDim(0.06), Label: LabelLegit},
		{Vector: withFirstDim(0.07), Label: LabelFraud},
		{Vector: withFirstDim(0.08), Label: LabelLegit},
		{Vector: withFirstDim(0.09), Label: LabelFraud},
	}, IVFBuildOptions{Clusters: 1, NProbe: 1, AmbiguousNProbe: 1, Repair: true})
	if err != nil {
		t.Fatal(err)
	}
	query := QuantizeVector(withFirstDim(0.035))
	block := unsafe.Pointer(unsafe.SliceData(idx.Blocks))
	var got [8]int64
	quantizedBlock8DistancesAVX2(&query[0], block, &got[0])
	for lane := 0; lane < 8; lane++ {
		want := quantizedBlockLaneDistanceUnsafe(query, block, lane, maxInt64Value)
		if got[lane] != want {
			t.Fatalf("lane %d distance = %d, want %d", lane, got[lane], want)
		}
	}
}

func TestAVX2Block32DistancesMatchScalar(t *testing.T) {
	if !useIVFAVX2 {
		t.Skip("AVX2 unavailable")
	}
	refs := make([]Reference, 32)
	for i := range refs {
		refs[i] = Reference{Vector: withFirstDim(float32(i+1) / 100), Label: LabelLegit}
		if i%3 == 0 {
			refs[i].Label = LabelFraud
		}
	}
	idx, err := BuildIVFIndex(refs, IVFBuildOptions{Clusters: 1, NProbe: 1, AmbiguousNProbe: 1, Repair: true})
	if err != nil {
		t.Fatal(err)
	}
	query := QuantizeVector(withFirstDim(0.175))
	block := unsafe.Pointer(unsafe.SliceData(idx.Blocks))
	var got [32]int64
	quantizedBlock32DistancesAVX2(&query[0], block, &got[0])
	for blockIndex := 0; blockIndex < 4; blockIndex++ {
		blockPtr := unsafe.Add(block, blockIndex*ivfBlockStride*2)
		for lane := 0; lane < ivfBlockSize; lane++ {
			outLane := blockIndex*ivfBlockSize + lane
			want := quantizedBlockLaneDistanceUnsafe(query, blockPtr, lane, maxInt64Value)
			if got[outLane] != want {
				t.Fatalf("block %d lane %d distance = %d, want %d", blockIndex, lane, got[outLane], want)
			}
		}
	}
}

func TestAVX2Block32ScanMatchesScalarWithPartialBlock(t *testing.T) {
	if !useIVFAVX2 {
		t.Skip("AVX2 unavailable")
	}
	refs := make([]Reference, 29)
	for i := range refs {
		refs[i] = Reference{Vector: withFirstDim(float32((i*7)%29) / 29), Label: LabelLegit}
		if i%4 == 0 || i%9 == 0 {
			refs[i].Label = LabelFraud
		}
	}
	idx, err := BuildIVFIndex(refs, IVFBuildOptions{Clusters: 1, NProbe: 1, AmbiguousNProbe: 1, Repair: true})
	if err != nil {
		t.Fatal(err)
	}
	query := QuantizeVector(withFirstDim(0.37))

	avxState := newIVFSearchState()
	idx.scanIVFBlocksAVX2(query, 0, &avxState)
	scalarState := newIVFSearchState()
	idx.scanIVFBlocksUnsafe(query, 0, &scalarState)

	if avxState.bestDist != scalarState.bestDist || avxState.bestFraud != scalarState.bestFraud || avxState.bestID != scalarState.bestID {
		t.Fatalf("block32 scan state mismatch\navx:    dist=%v fraud=%v id=%v\nscalar: dist=%v fraud=%v id=%v", avxState.bestDist, avxState.bestFraud, avxState.bestID, scalarState.bestDist, scalarState.bestFraud, scalarState.bestID)
	}
}

func TestAVX2CentroidDistancesMatchScalar(t *testing.T) {
	if !useIVFAVX2 {
		t.Skip("AVX2 unavailable")
	}
	refs := []Reference{
		{Vector: withFirstDim(0.01), Label: LabelFraud},
		{Vector: withFirstDim(0.02), Label: LabelLegit},
		{Vector: withFirstDim(0.03), Label: LabelFraud},
		{Vector: withFirstDim(0.04), Label: LabelLegit},
		{Vector: withFirstDim(0.05), Label: LabelFraud},
		{Vector: withFirstDim(0.06), Label: LabelLegit},
		{Vector: withFirstDim(0.07), Label: LabelFraud},
		{Vector: withFirstDim(0.08), Label: LabelLegit},
		{Vector: withFirstDim(0.09), Label: LabelFraud},
		{Vector: withFirstDim(0.10), Label: LabelLegit},
		{Vector: withFirstDim(0.11), Label: LabelFraud},
		{Vector: withFirstDim(0.12), Label: LabelLegit},
		{Vector: withFirstDim(0.13), Label: LabelFraud},
		{Vector: withFirstDim(0.14), Label: LabelLegit},
		{Vector: withFirstDim(0.15), Label: LabelFraud},
		{Vector: withFirstDim(0.16), Label: LabelLegit},
	}
	idx, err := BuildIVFIndex(refs, IVFBuildOptions{Clusters: 16, NProbe: 8, AmbiguousNProbe: 8, Repair: true})
	if err != nil {
		t.Fatal(err)
	}
	query := QuantizeVector(withFirstDim(0.035))
	centroids := unsafe.Pointer(unsafe.SliceData(idx.IVF.CentroidBlocks))
	var got [8]int64
	quantizedBlock8DistancesAVX2(&query[0], centroids, &got[0])
	for lane := 0; lane < 8; lane++ {
		want := quantizedDistance(query, idx.IVF.Centroids[lane*Dimensions:(lane+1)*Dimensions], maxInt64Value)
		if got[lane] != want {
			t.Fatalf("lane %d distance = %d, want %d", lane, got[lane], want)
		}
	}
	var scalarOut, avxOut [maxIVFProbe]uint32
	if idx.topIVFCentroidsScalar(query, 8, &scalarOut) != idx.topIVFCentroidsAVX2(query, 8, &avxOut) {
		t.Fatal("probe count mismatch")
	}
	for i := 0; i < 8; i++ {
		if scalarOut[i] != avxOut[i] {
			t.Fatalf("probe[%d] = %d, want %d", i, avxOut[i], scalarOut[i])
		}
	}
}

func TestCentroidBlocksAreMaterializedFromRowMajorCentroids(t *testing.T) {
	centroids := make([]int16, 9*Dimensions)
	for c := 0; c < 9; c++ {
		for d := 0; d < Dimensions; d++ {
			centroids[c*Dimensions+d] = int16(c*100 + d)
		}
	}
	idx := NewIVFQuantizedIndex(nil, nil, IVFMetadata{
		Clusters:    9,
		Centroids:   centroids,
		ListOffsets: make([]uint32, 10),
	})
	if got, want := len(idx.IVF.CentroidBlocks), blocksForRows(9)*ivfBlockStride; got != want {
		t.Fatalf("centroid block length = %d, want %d", got, want)
	}
	for c := 0; c < 9; c++ {
		block := c / ivfBlockSize
		lane := c % ivfBlockSize
		for d := 0; d < Dimensions; d++ {
			got := idx.IVF.CentroidBlocks[block*ivfBlockStride+d*ivfBlockSize+lane]
			want := centroids[c*Dimensions+d]
			if got != want {
				t.Fatalf("centroid block c=%d d=%d = %d, want %d", c, d, got, want)
			}
		}
	}
}

func TestAVX2CentroidSelectionFallsBackWithoutCentroidBlocks(t *testing.T) {
	if !useIVFAVX2 {
		t.Skip("AVX2 unavailable")
	}
	refs := []Reference{
		{Vector: withFirstDim(0.01), Label: LabelFraud},
		{Vector: withFirstDim(0.02), Label: LabelLegit},
		{Vector: withFirstDim(0.03), Label: LabelFraud},
		{Vector: withFirstDim(0.04), Label: LabelLegit},
		{Vector: withFirstDim(0.05), Label: LabelFraud},
		{Vector: withFirstDim(0.06), Label: LabelLegit},
		{Vector: withFirstDim(0.07), Label: LabelFraud},
		{Vector: withFirstDim(0.08), Label: LabelLegit},
	}
	idx, err := BuildIVFIndex(refs, IVFBuildOptions{Clusters: 8, NProbe: 4, AmbiguousNProbe: 4, Repair: true})
	if err != nil {
		t.Fatal(err)
	}
	idx.IVF.CentroidBlocks = nil
	query := QuantizeVector(withFirstDim(0.035))
	var scalarOut, avxOut [maxIVFProbe]uint32
	if idx.topIVFCentroidsScalar(query, 4, &scalarOut) != idx.topIVFCentroidsAVX2(query, 4, &avxOut) {
		t.Fatal("probe count mismatch")
	}
	for i := 0; i < 4; i++ {
		if scalarOut[i] != avxOut[i] {
			t.Fatalf("probe[%d] = %d, want %d", i, avxOut[i], scalarOut[i])
		}
	}
}

func TestHighRiskApprovalRepairRunsExactIVFSearch(t *testing.T) {
	vectors := make([]int16, 6*Dimensions)
	labels := []uint8{0, 0, 1, 1, 1, 0}
	for i := 0; i < 5; i++ {
		vectors[i*Dimensions] = int16(100 + i*10)
	}
	vectors[5*Dimensions] = 9000
	centroids := make([]int16, 2*Dimensions)
	centroids[0] = 9000
	centroids[Dimensions] = 100
	idx := NewIVFQuantizedIndex(vectors, labels, IVFMetadata{
		Clusters:        2,
		Centroids:       centroids,
		ListOffsets:     []uint32{0, 1, 6},
		BBoxMin:         make([]int16, 2*Dimensions),
		BBoxMax:         make([]int16, 2*Dimensions),
		OrigIDs:         []uint32{5, 0, 1, 2, 3, 4},
		NProbe:          1,
		AmbiguousNProbe: 1,
		Repair:          true,
	})
	query := [Dimensions]int16{
		0:  0,
		2:  10000,
		7:  3200,
		9:  QuantScale,
		10: 0,
		12: 8500,
	}

	if got := idx.Search5Quantized(query); got != 3 {
		t.Fatalf("frauds among nearest = %d, want 3", got)
	}
}
