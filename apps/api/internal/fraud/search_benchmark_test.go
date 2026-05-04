package fraud

import "testing"

func BenchmarkQuantizedIndexSearch5(b *testing.B) {
	refs := make([]Reference, 20000)
	for i := range refs {
		v := float32(i%10000) / 10000
		refs[i] = Reference{Vector: [Dimensions]float32{v, 0.25, 0.5, 0.75, 0.1, -1, -1, v, 0.2, 1, 0, 1, 0.75, 0.01}, Label: LabelLegit}
		if i%7 == 0 {
			refs[i].Label = LabelFraud
		}
	}
	idx, err := BuildIVFIndex(refs, IVFBuildOptions{Clusters: 256, NProbe: 8, AmbiguousNProbe: 32, Repair: true})
	if err != nil {
		b.Fatal(err)
	}
	query := refs[len(refs)/2].Vector

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = idx.Search5(query)
	}
}
