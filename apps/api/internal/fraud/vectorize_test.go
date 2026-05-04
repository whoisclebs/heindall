package fraud

import (
	"math"
	"testing"
	"time"
)

func TestVectorizeLegitDocumentationExample(t *testing.T) {
	req := TransactionRequest{
		ID:          "tx-1329056812",
		Transaction: Transaction{Amount: 41.12, Installments: 2, RequestedAt: mustTime(t, "2026-03-11T18:45:53Z")},
		Customer:    Customer{AvgAmount: 82.24, TxCount24h: 3, KnownMerchants: []string{"MERC-003", "MERC-016"}},
		Merchant:    Merchant{ID: "MERC-016", MCC: "5411", AvgAmount: 60.25},
		Terminal:    Terminal{IsOnline: false, CardPresent: true, KmFromHome: 29.23},
	}

	got := Vectorize(req, DefaultNormalization(), DefaultMCCRisk())
	want := [Dimensions]float32{0.0041, 0.1667, 0.05, 0.7826, 0.3333, -1, -1, 0.0292, 0.15, 0, 1, 0, 0.15, 0.006}
	assertVectorNear(t, got, want, 0.0001)
}

func TestVectorizeJSONMatchesStructVectorizer(t *testing.T) {
	payload := []byte(`{"id":"tx-1329056812","transaction":{"amount":41.12,"installments":2,"requested_at":"2026-03-11T18:45:53Z"},"customer":{"avg_amount":82.24,"tx_count_24h":3,"known_merchants":["MERC-003","MERC-016"]},"merchant":{"id":"MERC-016","mcc":"5411","avg_amount":60.25},"terminal":{"is_online":false,"card_present":true,"km_from_home":29.23},"last_transaction":null}`)
	req := TransactionRequest{
		ID:          "tx-1329056812",
		Transaction: Transaction{Amount: 41.12, Installments: 2, RequestedAt: mustTime(t, "2026-03-11T18:45:53Z")},
		Customer:    Customer{AvgAmount: 82.24, TxCount24h: 3, KnownMerchants: []string{"MERC-003", "MERC-016"}},
		Merchant:    Merchant{ID: "MERC-016", MCC: "5411", AvgAmount: 60.25},
		Terminal:    Terminal{IsOnline: false, CardPresent: true, KmFromHome: 29.23},
	}

	got, ok := VectorizeJSON(payload, DefaultNormalization(), DefaultMCCRisk())
	if !ok {
		t.Fatal("VectorizeJSON returned false")
	}
	want := Vectorize(req, DefaultNormalization(), DefaultMCCRisk())
	assertVectorNear(t, got, want, 0.0001)
}

func TestVectorizeJSONQuantizedMatchesQuantizedStructVectorizer(t *testing.T) {
	payload := []byte(`{"id":"tx-1329056812","transaction":{"amount":41.12,"installments":2,"requested_at":"2026-03-11T18:45:53Z"},"customer":{"avg_amount":82.24,"tx_count_24h":3,"known_merchants":["MERC-003","MERC-016"]},"merchant":{"id":"MERC-016","mcc":"5411","avg_amount":60.25},"terminal":{"is_online":false,"card_present":true,"km_from_home":29.23},"last_transaction":null}`)
	req := TransactionRequest{
		ID:          "tx-1329056812",
		Transaction: Transaction{Amount: 41.12, Installments: 2, RequestedAt: mustTime(t, "2026-03-11T18:45:53Z")},
		Customer:    Customer{AvgAmount: 82.24, TxCount24h: 3, KnownMerchants: []string{"MERC-003", "MERC-016"}},
		Merchant:    Merchant{ID: "MERC-016", MCC: "5411", AvgAmount: 60.25},
		Terminal:    Terminal{IsOnline: false, CardPresent: true, KmFromHome: 29.23},
	}

	got, ok := VectorizeJSONQuantized(payload, DefaultNormalization())
	if !ok {
		t.Fatal("VectorizeJSONQuantized returned false")
	}
	want := QuantizeVector(Vectorize(req, DefaultNormalization(), DefaultMCCRisk()))
	if got != want {
		t.Fatalf("quantized vector = %#v, want %#v", got, want)
	}
}

func TestVectorizeJSONQuantizedHandlesReorderedTopLevelObjects(t *testing.T) {
	payload := []byte(`{"id":"tx-1329056812","transaction":{"amount":41.12,"installments":2,"requested_at":"2026-03-11T18:45:53Z"},"merchant":{"id":"MERC-016","mcc":"5411","avg_amount":60.25},"customer":{"avg_amount":82.24,"tx_count_24h":3,"known_merchants":["MERC-003","MERC-016"]},"terminal":{"is_online":false,"card_present":true,"km_from_home":29.23},"last_transaction":null}`)
	req := TransactionRequest{
		ID:          "tx-1329056812",
		Transaction: Transaction{Amount: 41.12, Installments: 2, RequestedAt: mustTime(t, "2026-03-11T18:45:53Z")},
		Customer:    Customer{AvgAmount: 82.24, TxCount24h: 3, KnownMerchants: []string{"MERC-003", "MERC-016"}},
		Merchant:    Merchant{ID: "MERC-016", MCC: "5411", AvgAmount: 60.25},
		Terminal:    Terminal{IsOnline: false, CardPresent: true, KmFromHome: 29.23},
	}

	got, ok := VectorizeJSONQuantized(payload, DefaultNormalization())
	if !ok {
		t.Fatal("VectorizeJSONQuantized returned false")
	}
	want := QuantizeVector(Vectorize(req, DefaultNormalization(), DefaultMCCRisk()))
	if got != want {
		t.Fatalf("quantized vector = %#v, want %#v", got, want)
	}
}

func TestVectorizeFraudDocumentationExample(t *testing.T) {
	req := TransactionRequest{
		ID:          "tx-3330991687",
		Transaction: Transaction{Amount: 9505.97, Installments: 10, RequestedAt: mustTime(t, "2026-03-14T05:15:12Z")},
		Customer:    Customer{AvgAmount: 81.28, TxCount24h: 20, KnownMerchants: []string{"MERC-008", "MERC-007", "MERC-005"}},
		Merchant:    Merchant{ID: "MERC-068", MCC: "7802", AvgAmount: 54.86},
		Terminal:    Terminal{IsOnline: false, CardPresent: true, KmFromHome: 952.27},
	}

	got := Vectorize(req, DefaultNormalization(), DefaultMCCRisk())
	want := [Dimensions]float32{0.9506, 0.8333, 1, 0.2174, 0.8333, -1, -1, 0.9523, 1, 0, 1, 1, 0.75, 0.0055}
	assertVectorNear(t, got, want, 0.0001)
}

func TestExactSearcherReturnsFraudCountAmongFiveNearest(t *testing.T) {
	query := [Dimensions]float32{}
	refs := []Reference{
		{Vector: withFirstDim(0.01), Label: LabelFraud},
		{Vector: withFirstDim(0.02), Label: LabelLegit},
		{Vector: withFirstDim(0.03), Label: LabelFraud},
		{Vector: withFirstDim(0.04), Label: LabelLegit},
		{Vector: withFirstDim(0.05), Label: LabelFraud},
		{Vector: withFirstDim(9.00), Label: LabelFraud},
	}

	got := NewExactSearcher(refs).Search5(query)
	if got != 3 {
		t.Fatalf("frauds among nearest = %d, want 3", got)
	}
}

func mustTime(t *testing.T, value string) time.Time {
	t.Helper()
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}

func assertVectorNear(t *testing.T, got, want [Dimensions]float32, tolerance float64) {
	t.Helper()
	for i := range want {
		if math.Abs(float64(got[i]-want[i])) > tolerance {
			t.Fatalf("vector[%d] = %.6f, want %.6f", i, got[i], want[i])
		}
	}
}

func withFirstDim(v float32) [Dimensions]float32 {
	vec := [Dimensions]float32{}
	vec[0] = v
	return vec
}
