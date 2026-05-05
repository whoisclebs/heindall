package router

import (
	"bytes"
	"net/http/httptest"
	"testing"

	"github.com/whoisclebs/heindall/apps/api/internal/fraud"
)

func benchmarkFraudPayload() []byte {
	return []byte(`{"id":"tx-1329056812","transaction":{"amount":2575.75,"installments":7,"requested_at":"2026-03-27T09:08:20Z"},"customer":{"avg_amount":217.64,"tx_count_24h":7,"known_merchants":["MERC-005","MERC-003","MERC-001","MERC-011"]},"merchant":{"id":"MERC-003","mcc":"7802","avg_amount":293.16},"terminal":{"is_online":true,"card_present":false,"km_from_home":397.3207551537},"last_transaction":{"timestamp":"2026-03-27T07:53:20Z","km_from_current":51.1775721868}}`)
}

func benchmarkFraudService(b *testing.B) *fraud.Service {
	b.Helper()
	refs := make([]fraud.Reference, 20000)
	for i := range refs {
		v := float32(i%10000) / 10000
		refs[i] = fraud.Reference{Vector: [fraud.Dimensions]float32{v, 0.25, 0.5, 0.75, 0.1, -1, -1, v, 0.2, 1, 0, 1, 0.75, 0.01}, Label: fraud.LabelLegit}
		if i%7 == 0 {
			refs[i].Label = fraud.LabelFraud
		}
	}
	idx, err := fraud.BuildIVFIndex(refs, fraud.IVFBuildOptions{Clusters: 256, NProbe: 8, AmbiguousNProbe: 24, Repair: true})
	if err != nil {
		b.Fatal(err)
	}
	return fraud.NewService(fraud.DefaultNormalization(), fraud.DefaultMCCRisk(), idx)
}

func BenchmarkFraudScoreRawHandler(b *testing.B) {
	h := NewHandlers(benchmarkFraudService(b), 4096)
	payload := benchmarkFraudPayload()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("POST", "/fraud-score", bytes.NewReader(payload))
		req.ContentLength = int64(len(payload))
		rw := httptest.NewRecorder()
		h.FraudScoreRaw(rw, req)
		if rw.Code != 200 {
			b.Fatalf("status = %d", rw.Code)
		}
	}
}
