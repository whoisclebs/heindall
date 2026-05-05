package fraud

import "testing"

var benchmarkVectorSink [Dimensions]int16

func benchmarkPayload() []byte {
	return []byte(`{"id":"tx-1329056812","transaction":{"amount":2575.75,"installments":7,"requested_at":"2026-03-27T09:08:20Z"},"customer":{"avg_amount":217.64,"tx_count_24h":7,"known_merchants":["MERC-005","MERC-003","MERC-001","MERC-011"]},"merchant":{"id":"MERC-003","mcc":"7802","avg_amount":293.16},"terminal":{"is_online":true,"card_present":false,"km_from_home":397.3207551537},"last_transaction":{"timestamp":"2026-03-27T07:53:20Z","km_from_current":51.1775721868}}`)
}

func BenchmarkVectorizeJSONQuantized(b *testing.B) {
	payload := benchmarkPayload()
	norm := DefaultNormalization()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		vec, ok := VectorizeJSONQuantized(payload, norm)
		if !ok {
			b.Fatal("VectorizeJSONQuantized returned false")
		}
		benchmarkVectorSink = vec
	}
}

func BenchmarkFraudCountJSON(b *testing.B) {
	idx, _, _ := buildBenchmarkIVFIndex(b)
	svc := NewService(DefaultNormalization(), DefaultMCCRisk(), idx)
	payload := benchmarkPayload()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		frauds, ok := svc.FraudCountJSON(payload)
		if !ok {
			b.Fatal("FraudCountJSON returned false")
		}
		benchmarkFraudSink = frauds
	}
}
