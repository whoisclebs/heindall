package fraud

import (
	"testing"
	"unsafe"
)

var benchmarkFraudSink int
var benchmarkProbeSink [maxIVFProbe]uint32
var benchmarkClusterSink int

func buildBenchmarkIVFIndex(b *testing.B) (*QuantizedIndex, [Dimensions]int16, [Dimensions]float32) {
	b.Helper()
	refs := make([]Reference, 20000)
	for i := range refs {
		v := float32(i%10000) / 10000
		refs[i] = Reference{Vector: [Dimensions]float32{v, 0.25, 0.5, 0.75, 0.1, -1, -1, v, 0.2, 1, 0, 1, 0.75, 0.01}, Label: LabelLegit}
		if i%7 == 0 {
			refs[i].Label = LabelFraud
		}
	}
	idx, err := BuildIVFIndex(refs, IVFBuildOptions{Clusters: 256, NProbe: 8, AmbiguousNProbe: 24, Repair: true})
	if err != nil {
		b.Fatal(err)
	}
	queryFloat := refs[len(refs)/2].Vector
	query := QuantizeVector(queryFloat)
	return idx, query, queryFloat
}

func rowMajorBenchmarkIndex(idx *QuantizedIndex) *QuantizedIndex {
	clone := *idx
	clone.Blocks = nil
	clone.IVF.BlockOffsets = nil
	return &clone
}

func largestIVFListCluster(idx *QuantizedIndex) int {
	cluster := 0
	best := uint32(0)
	for c := 0; c < idx.IVF.Clusters; c++ {
		span := idx.IVF.ListOffsets[c+1] - idx.IVF.ListOffsets[c]
		if span > best {
			best = span
			cluster = c
		}
	}
	return cluster
}

func benchmarkRepairSeedState(idx *QuantizedIndex, query [Dimensions]int16) ivfSearchState {
	var probeIDs [maxIVFProbe]uint32
	probeCount := idx.topIVFCentroids(query, idx.IVF.NProbe, &probeIDs)
	state := newIVFSearchState()
	for i := 0; i < probeCount; i++ {
		state.addProbe(probeIDs[i])
		idx.scanIVFList(query, int(probeIDs[i]), &state)
	}
	return state
}

func repairIVFInstrumented(idx *QuantizedIndex, query [Dimensions]int16, state *ivfSearchState, scan bool) int {
	if len(idx.IVF.BBoxMin) < idx.IVF.Clusters*Dimensions || len(idx.IVF.BBoxMax) < idx.IVF.Clusters*Dimensions {
		return 0
	}
	cutoff := state.bestDist[4]
	if cutoff == maxInt64Value {
		return 0
	}
	clusters := 0
	for c := 0; c < idx.IVF.Clusters; c++ {
		if state.hasProbe(uint32(c)) {
			continue
		}
		if idx.ivfBBoxDistance(query, c, cutoff) <= cutoff {
			clusters++
			if scan {
				idx.scanIVFList(query, c, state)
			}
		}
	}
	return clusters
}

func repairStressState(idx *QuantizedIndex) ivfSearchState {
	state := newIVFSearchState()
	for i := 0; i < 5; i++ {
		state.bestDist[i] = maxInt64Value / 4
		state.bestID[i] = maxUint32Value
	}
	return state
}

func forcedFallbackSearch(idx *QuantizedIndex, query [Dimensions]int16, nprobe, ambiguousNProbe int, repair bool) int {
	frauds, state := idx.searchIVFProbes(query, nprobe, false)
	if ambiguousNProbe > nprobe {
		idx.expandIVFProbes(query, &state, ambiguousNProbe)
		if repair {
			idx.repairIVF(query, &state)
		}
		frauds = state.countFrauds()
	} else if repair {
		idx.repairIVF(query, &state)
		frauds = state.countFrauds()
	}
	return frauds
}

func BenchmarkQuantizedIndexSearch5(b *testing.B) {
	idx, _, query := buildBenchmarkIVFIndex(b)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchmarkFraudSink = idx.Search5(query)
	}
}

func BenchmarkTopIVFCentroids(b *testing.B) {
	idx, query, _ := buildBenchmarkIVFIndex(b)
	var out [maxIVFProbe]uint32

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = idx.topIVFCentroids(query, idx.IVF.NProbe, &out)
	}
	benchmarkProbeSink = out
}

func BenchmarkTopIVFCentroidsScalar(b *testing.B) {
	idx, query, _ := buildBenchmarkIVFIndex(b)
	var out [maxIVFProbe]uint32

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = idx.topIVFCentroidsScalar(query, idx.IVF.NProbe, &out)
	}
	benchmarkProbeSink = out
}

func BenchmarkTopIVFCentroidsAVX2(b *testing.B) {
	if !useIVFAVX2 {
		b.Skip("AVX2 unavailable")
	}
	idx, query, _ := buildBenchmarkIVFIndex(b)
	var out [maxIVFProbe]uint32

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = idx.topIVFCentroidsAVX2(query, idx.IVF.NProbe, &out)
	}
	benchmarkProbeSink = out
}

func BenchmarkScanIVFListRowMajor(b *testing.B) {
	idx, query, _ := buildBenchmarkIVFIndex(b)
	idx = rowMajorBenchmarkIndex(idx)
	cluster := largestIVFListCluster(idx)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		state := newIVFSearchState()
		idx.scanIVFList(query, cluster, &state)
		benchmarkFraudSink = state.countFrauds()
	}
}

func BenchmarkScanIVFListBlock8(b *testing.B) {
	idx, query, _ := buildBenchmarkIVFIndex(b)
	cluster := largestIVFListCluster(idx)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		state := newIVFSearchState()
		idx.scanIVFList(query, cluster, &state)
		benchmarkFraudSink = state.countFrauds()
	}
}

func BenchmarkScanIVFListBlock8Scalar(b *testing.B) {
	idx, query, _ := buildBenchmarkIVFIndex(b)
	cluster := largestIVFListCluster(idx)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		state := newIVFSearchState()
		idx.scanIVFBlocksUnsafe(query, cluster, &state)
		benchmarkFraudSink = state.countFrauds()
	}
}

func BenchmarkScanIVFListBlock8AVX2(b *testing.B) {
	idx, query, _ := buildBenchmarkIVFIndex(b)
	cluster := largestIVFListCluster(idx)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		state := newIVFSearchState()
		idx.scanIVFBlocksAVX2(query, cluster, &state)
		benchmarkFraudSink = state.countFrauds()
	}
}

func BenchmarkBlock8DistancesAVX2Prototype(b *testing.B) {
	idx, query, _ := buildBenchmarkIVFIndex(b)
	cluster := largestIVFListCluster(idx)
	blockStart := int(idx.IVF.BlockOffsets[cluster])
	block := unsafe.Pointer(unsafe.SliceData(idx.Blocks))
	block = unsafe.Add(block, blockStart*ivfBlockStride*2)
	var out [8]int64

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		quantizedBlock8DistancesAVX2(&query[0], block, &out[0])
	}
	benchmarkFraudSink = int(out[0])
}

func BenchmarkBlock32DistancesAVX2Prototype(b *testing.B) {
	if !useIVFAVX2 {
		b.Skip("AVX2 unavailable")
	}
	idx, query, _ := buildBenchmarkIVFIndex(b)
	cluster := largestIVFListCluster(idx)
	blockStart := int(idx.IVF.BlockOffsets[cluster])
	block := unsafe.Pointer(unsafe.SliceData(idx.Blocks))
	block = unsafe.Add(block, blockStart*ivfBlockStride*2)
	var out [32]int64

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		quantizedBlock32DistancesAVX2(&query[0], block, &out[0])
	}
	benchmarkFraudSink = int(out[0])
}

func BenchmarkRepairIVF(b *testing.B) {
	idx, query, _ := buildBenchmarkIVFIndex(b)
	seed := benchmarkRepairSeedState(idx, query)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		state := seed
		idx.repairIVF(query, &state)
		benchmarkFraudSink = state.countFrauds()
	}
}

func BenchmarkRepairIVFBBoxOnly(b *testing.B) {
	idx, query, _ := buildBenchmarkIVFIndex(b)
	seed := benchmarkRepairSeedState(idx, query)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		state := seed
		benchmarkClusterSink = repairIVFInstrumented(idx, query, &state, false)
	}
}

func BenchmarkRepairIVFWithExtraScans(b *testing.B) {
	idx, query, _ := buildBenchmarkIVFIndex(b)
	seed := repairStressState(idx)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		state := seed
		benchmarkClusterSink = repairIVFInstrumented(idx, query, &state, true)
		benchmarkFraudSink = state.countFrauds()
	}
}

func BenchmarkFallbackNProbe8Only(b *testing.B) {
	idx, query, _ := buildBenchmarkIVFIndex(b)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		frauds, _ := idx.searchIVFProbes(query, 8, false)
		benchmarkFraudSink = frauds
	}
}

func BenchmarkFallbackForced8To24NoRepair(b *testing.B) {
	idx, query, _ := buildBenchmarkIVFIndex(b)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchmarkFraudSink = forcedFallbackSearch(idx, query, 8, 24, false)
	}
}

func BenchmarkFallbackForced8To24Repair(b *testing.B) {
	idx, query, _ := buildBenchmarkIVFIndex(b)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchmarkFraudSink = forcedFallbackSearch(idx, query, 8, 24, true)
	}
}

func BenchmarkSearch5Quantized(b *testing.B) {
	idx, query, _ := buildBenchmarkIVFIndex(b)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchmarkFraudSink = idx.Search5Quantized(query)
	}
}

func BenchmarkIVFMixedPipeline(b *testing.B) {
	idx, query, queryFloat := buildBenchmarkIVFIndex(b)
	rowMajor := rowMajorBenchmarkIndex(idx)
	cluster := largestIVFListCluster(idx)
	seed := benchmarkRepairSeedState(idx, query)
	var out [maxIVFProbe]uint32

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		frauds := idx.topIVFCentroids(query, idx.IVF.NProbe, &out)

		rowState := newIVFSearchState()
		rowMajor.scanIVFList(query, cluster, &rowState)
		frauds += rowState.countFrauds()

		blockState := newIVFSearchState()
		idx.scanIVFList(query, cluster, &blockState)
		frauds += blockState.countFrauds()

		repairState := seed
		idx.repairIVF(query, &repairState)
		frauds += repairState.countFrauds()

		frauds += idx.Search5Quantized(query)
		frauds += idx.Search5(queryFloat)
		benchmarkFraudSink = frauds
	}
	benchmarkProbeSink = out
}
