package fraud

import "testing"

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
