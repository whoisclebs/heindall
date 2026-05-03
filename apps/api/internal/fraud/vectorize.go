package fraud

import "time"

type Normalization struct {
	MaxAmount            float64 `json:"max_amount"`
	MaxInstallments      float64 `json:"max_installments"`
	AmountVsAvgRatio     float64 `json:"amount_vs_avg_ratio"`
	MaxMinutes           float64 `json:"max_minutes"`
	MaxKM                float64 `json:"max_km"`
	MaxTxCount24h        float64 `json:"max_tx_count_24h"`
	MaxMerchantAvgAmount float64 `json:"max_merchant_avg_amount"`
}

func DefaultNormalization() Normalization {
	return Normalization{
		MaxAmount:            10000,
		MaxInstallments:      12,
		AmountVsAvgRatio:     10,
		MaxMinutes:           1440,
		MaxKM:                1000,
		MaxTxCount24h:        20,
		MaxMerchantAvgAmount: 10000,
	}
}

func DefaultMCCRisk() map[string]float64 {
	return map[string]float64{
		"5411": 0.15,
		"5812": 0.30,
		"5912": 0.20,
		"5944": 0.45,
		"7801": 0.80,
		"7802": 0.75,
		"7995": 0.85,
		"4511": 0.35,
		"5311": 0.25,
		"5999": 0.50,
	}
}

func Vectorize(req TransactionRequest, norm Normalization, mccRisk map[string]float64) [Dimensions]float32 {
	requestedAt := req.Transaction.RequestedAt.UTC()
	vec := [Dimensions]float32{}

	vec[0] = f32(clamp(div(req.Transaction.Amount, norm.MaxAmount)))
	vec[1] = f32(clamp(div(float64(req.Transaction.Installments), norm.MaxInstallments)))
	vec[2] = f32(clamp(div(div(req.Transaction.Amount, req.Customer.AvgAmount), norm.AmountVsAvgRatio)))
	vec[3] = f32(float64(requestedAt.Hour()) / 23.0)
	vec[4] = f32(float64(mondayBasedWeekday(requestedAt)) / 6.0)

	if req.LastTransaction == nil {
		vec[5] = -1
		vec[6] = -1
	} else {
		minutes := requestedAt.Sub(req.LastTransaction.Timestamp.UTC()).Minutes()
		vec[5] = f32(clamp(div(minutes, norm.MaxMinutes)))
		vec[6] = f32(clamp(div(req.LastTransaction.KmFromCurrent, norm.MaxKM)))
	}

	vec[7] = f32(clamp(div(req.Terminal.KmFromHome, norm.MaxKM)))
	vec[8] = f32(clamp(div(float64(req.Customer.TxCount24h), norm.MaxTxCount24h)))
	vec[9] = boolFloat(req.Terminal.IsOnline)
	vec[10] = boolFloat(req.Terminal.CardPresent)
	vec[11] = boolFloat(!knownMerchant(req.Merchant.ID, req.Customer.KnownMerchants))
	vec[12] = f32(mccRiskValue(req.Merchant.MCC, mccRisk))
	vec[13] = f32(clamp(div(req.Merchant.AvgAmount, norm.MaxMerchantAvgAmount)))

	return vec
}

func mondayBasedWeekday(t time.Time) int {
	return (int(t.Weekday()) + 6) % 7
}

func knownMerchant(id string, merchants []string) bool {
	for _, merchant := range merchants {
		if merchant == id {
			return true
		}
	}
	return false
}

func mccRiskValue(mcc string, risk map[string]float64) float64 {
	if v, ok := risk[mcc]; ok {
		return v
	}
	return 0.5
}

func div(n, d float64) float64 {
	if d <= 0 {
		return 1
	}
	return n / d
}

func clamp(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func boolFloat(v bool) float32 {
	if v {
		return 1
	}
	return 0
}

func f32(v float64) float32 { return float32(v) }
