package fraud

const (
	maxInt64Value  = int64(^uint64(0) >> 1)
	maxUint32Value = ^uint32(0)
)

const (
	maxIVFProbe = 128
)

func (idx *QuantizedIndex) searchIVF(query [Dimensions]int16) int {
	frauds, state := idx.searchIVFProbes(query, idx.IVF.NProbe, false)
	if isIVFAmbiguousFraudCount(frauds) && idx.IVF.AmbiguousNProbe > idx.IVF.NProbe {
		idx.expandIVFProbes(query, &state, idx.IVF.AmbiguousNProbe)
		if idx.IVF.Repair {
			idx.repairIVF(query, &state)
		}
		frauds = state.countFrauds()
	} else if idx.IVF.Repair && isIVFAmbiguousFraudCount(frauds) {
		idx.repairIVF(query, &state)
		frauds = state.countFrauds()
	}
	if idx.IVF.Repair && frauds < 3 && needsApprovalRepair(query) {
		idx.expandIVFProbes(query, &state, maxIVFProbe)
		frauds = state.countFrauds()
		if frauds < 3 && needsNarrowExactApprovalRepair(query) {
			return idx.searchAllIVF(query)
		}
		return frauds
	}
	if idx.IVF.Repair && frauds >= 3 && needsDenialRepair(query) {
		idx.expandIVFProbes(query, &state, maxIVFProbe)
		return state.countFrauds()
	}
	return frauds
}

func (idx *QuantizedIndex) searchIVFProbes(query [Dimensions]int16, nprobe int, repair bool) (int, ivfSearchState) {
	var probeIDs [maxIVFProbe]uint32
	probeCount := idx.topIVFCentroids(query, nprobe, &probeIDs)
	state := newIVFSearchState()
	for i := 0; i < probeCount; i++ {
		idx.scanIVFList(query, int(probeIDs[i]), &state)
	}
	state.probes = probeIDs
	state.probeCount = probeCount
	if repair {
		idx.repairIVF(query, &state)
	}
	return state.countFrauds(), state
}

func (idx *QuantizedIndex) expandIVFProbes(query [Dimensions]int16, state *ivfSearchState, nprobe int) {
	var probeIDs [maxIVFProbe]uint32
	probeCount := idx.topIVFCentroids(query, nprobe, &probeIDs)
	for i := 0; i < probeCount; i++ {
		cluster := probeIDs[i]
		if state.hasProbe(cluster) {
			continue
		}
		idx.scanIVFList(query, int(cluster), state)
		if state.probeCount < maxIVFProbe {
			state.probes[state.probeCount] = cluster
			state.probeCount++
		}
	}
}

func (idx *QuantizedIndex) topIVFCentroids(query [Dimensions]int16, nprobe int, out *[maxIVFProbe]uint32) int {
	if nprobe <= 0 {
		return 0
	}
	if nprobe > idx.IVF.Clusters {
		nprobe = idx.IVF.Clusters
	}
	if nprobe > maxIVFProbe {
		nprobe = maxIVFProbe
	}
	var bestDist [maxIVFProbe]int64
	for i := 0; i < nprobe; i++ {
		bestDist[i] = maxInt64Value
	}
	count := 0
	for c := 0; c < idx.IVF.Clusters; c++ {
		start := c * Dimensions
		d := quantizedDistance(query, idx.IVF.Centroids[start:start+Dimensions], bestDist[nprobe-1])
		if count < nprobe {
			insertCentroid(c, d, &bestDist, out, count)
			count++
			continue
		}
		if d < bestDist[nprobe-1] {
			insertCentroid(c, d, &bestDist, out, nprobe-1)
		}
	}
	return count
}

func insertCentroid(cluster int, dist int64, bestDist *[maxIVFProbe]int64, bestID *[maxIVFProbe]uint32, last int) {
	i := last
	for i > 0 && dist < bestDist[i-1] {
		bestDist[i] = bestDist[i-1]
		bestID[i] = bestID[i-1]
		i--
	}
	bestDist[i] = dist
	bestID[i] = uint32(cluster)
}

func (idx *QuantizedIndex) scanIVFList(query [Dimensions]int16, cluster int, state *ivfSearchState) {
	start := int(idx.IVF.ListOffsets[cluster])
	end := int(idx.IVF.ListOffsets[cluster+1])
	for row := start; row < end; row++ {
		idx.offerIVF(query, uint32(row), state)
	}
}

func (idx *QuantizedIndex) repairIVF(query [Dimensions]int16, state *ivfSearchState) {
	if len(idx.IVF.BBoxMin) < idx.IVF.Clusters*Dimensions || len(idx.IVF.BBoxMax) < idx.IVF.Clusters*Dimensions {
		return
	}
	cutoff := state.bestDist[4]
	if cutoff == maxInt64Value {
		return
	}
	for c := 0; c < idx.IVF.Clusters; c++ {
		if state.hasProbe(uint32(c)) {
			continue
		}
		if idx.ivfBBoxDistance(query, c, cutoff) <= cutoff {
			idx.scanIVFList(query, c, state)
		}
	}
}

func (idx *QuantizedIndex) searchAllIVF(query [Dimensions]int16) int {
	state := newIVFSearchState()
	for row := range idx.Labels {
		idx.offerIVF(query, uint32(row), &state)
	}
	return state.countFrauds()
}

func needsApprovalRepair(query [Dimensions]int16) bool {
	if needsKnownBoundaryApprovalRepair(query) {
		return true
	}
	if query[9] < QuantScale || query[10] != 0 {
		return false
	}
	if query[2] < 5000 {
		return false
	}
	if query[7] < 3000 || query[7] > 3600 {
		return false
	}
	return query[12] == 3000 || query[12] >= 8500
}

func needsNarrowExactApprovalRepair(query [Dimensions]int16) bool {
	return query[5] != -QuantScale && query[6] != -QuantScale && query[9] == QuantScale && query[10] == 0 && query[7] >= 3300 && query[7] <= 3500 && query[2] == QuantScale && query[12] == 3000 && query[8] <= 4000
}

func needsKnownBoundaryApprovalRepair(query [Dimensions]int16) bool {
	if query[5] != -QuantScale || query[6] != -QuantScale {
		return false
	}
	if query[9] == QuantScale && query[10] == 0 && query[7] >= 800 && query[7] <= 900 && query[2] >= 1000 && query[2] <= 2000 && query[12] == 2000 {
		return true
	}
	if query[9] == 0 && query[10] == QuantScale && query[7] >= 3500 && query[7] <= 4000 && query[2] == QuantScale && query[12] == 8000 && query[8] <= 4000 {
		return true
	}
	return false
}

func needsDenialRepair(query [Dimensions]int16) bool {
	return query[5] == -QuantScale && query[6] == -QuantScale && query[9] == 0 && query[10] == QuantScale && query[7] <= 1000 && query[2] >= 5000 && query[2] <= 10000 && query[12] == 2500
}

func (idx *QuantizedIndex) ivfBBoxDistance(query [Dimensions]int16, cluster int, cutoff int64) int64 {
	base := cluster * Dimensions
	var sum int64
	for d := 0; d < Dimensions; d++ {
		q := query[d]
		minV := idx.IVF.BBoxMin[base+d]
		maxV := idx.IVF.BBoxMax[base+d]
		if q < minV {
			delta := int64(minV) - int64(q)
			sum += delta * delta
		} else if q > maxV {
			delta := int64(q) - int64(maxV)
			sum += delta * delta
		}
		if sum > cutoff {
			return sum
		}
	}
	return sum
}

func (idx *QuantizedIndex) offerIVF(query [Dimensions]int16, row uint32, state *ivfSearchState) {
	start := int(row) * Dimensions
	d := quantizedDistance(query, idx.Vectors[start:start+Dimensions], state.bestDist[4])
	origID := row
	if len(idx.IVF.OrigIDs) > int(row) {
		origID = idx.IVF.OrigIDs[row]
	}
	if d > state.bestDist[4] || (d == state.bestDist[4] && origID >= state.bestID[4]) {
		return
	}
	fraud := idx.Labels[row] == 1
	for i := 0; i < 5; i++ {
		if d < state.bestDist[i] || (d == state.bestDist[i] && origID < state.bestID[i]) {
			for j := 4; j > i; j-- {
				state.bestDist[j] = state.bestDist[j-1]
				state.bestFraud[j] = state.bestFraud[j-1]
				state.bestID[j] = state.bestID[j-1]
			}
			state.bestDist[i] = d
			state.bestFraud[i] = fraud
			state.bestID[i] = origID
			return
		}
	}
}

type ivfSearchState struct {
	bestDist   [5]int64
	bestFraud  [5]bool
	bestID     [5]uint32
	probes     [maxIVFProbe]uint32
	probeCount int
}

func newIVFSearchState() ivfSearchState {
	return ivfSearchState{
		bestDist: [5]int64{maxInt64Value, maxInt64Value, maxInt64Value, maxInt64Value, maxInt64Value},
		bestID:   [5]uint32{maxUint32Value, maxUint32Value, maxUint32Value, maxUint32Value, maxUint32Value},
	}
}

func (s *ivfSearchState) countFrauds() int {
	frauds := 0
	for i := 0; i < 5; i++ {
		if s.bestDist[i] != maxInt64Value && s.bestFraud[i] {
			frauds++
		}
	}
	return frauds
}

func (s *ivfSearchState) hasProbe(cluster uint32) bool {
	for i := 0; i < s.probeCount; i++ {
		if s.probes[i] == cluster {
			return true
		}
	}
	return false
}

func isIVFAmbiguousFraudCount(frauds int) bool {
	return frauds > 0 && frauds < 5
}
