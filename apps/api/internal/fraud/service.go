package fraud

type ScoreService interface {
	Score(req TransactionRequest) ScoreResponse
}

type Service struct {
	norm    Normalization
	mccRisk map[string]float64
	search  Searcher
}

func NewService(norm Normalization, mccRisk map[string]float64, search Searcher) *Service {
	return &Service{norm: norm, mccRisk: mccRisk, search: search}
}

func (s *Service) Score(req TransactionRequest) ScoreResponse {
	query := Vectorize(req, s.norm, s.mccRisk)
	frauds := s.search.Search5(query)
	score := float64(frauds) / 5.0
	return ScoreResponse{
		Approved:   score < 0.6,
		FraudScore: score,
	}
}
