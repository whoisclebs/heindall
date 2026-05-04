package fraud

type ScoreService interface {
	Score(req TransactionRequest) ScoreResponse
}

type Service struct {
	norm      Normalization
	mccRisk   map[string]float64
	search    Searcher
	quantized *QuantizedIndex
}

func NewService(norm Normalization, mccRisk map[string]float64, search Searcher) *Service {
	svc := &Service{norm: norm, mccRisk: mccRisk, search: search}
	if quantized, ok := search.(*QuantizedIndex); ok {
		svc.quantized = quantized
	}
	return svc
}

func (s *Service) FraudCount(req TransactionRequest) int {
	query := Vectorize(req, s.norm, s.mccRisk)
	return s.search.Search5(query)
}

func (s *Service) FraudCountJSON(data []byte) (int, bool) {
	if s.quantized != nil {
		query, ok := VectorizeJSONQuantized(data, s.norm)
		if !ok {
			return 0, false
		}
		return s.quantized.Search5Quantized(query), true
	}
	query, ok := VectorizeJSON(data, s.norm, s.mccRisk)
	if !ok {
		return 0, false
	}
	return s.search.Search5(query), true
}

func (s *Service) Score(req TransactionRequest) ScoreResponse {
	frauds := s.FraudCount(req)
	score := float64(frauds) / 5.0
	return ScoreResponse{
		Approved:   score < 0.6,
		FraudScore: score,
	}
}
