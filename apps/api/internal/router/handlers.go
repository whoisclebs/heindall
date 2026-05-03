package router

import (
	"net/http"

	"github.com/go-golpher/golpher"
	"github.com/whoisclebs/heindall/apps/api/internal/fraud"
)

type Handlers struct {
	scoreService fraud.ScoreService
}

func NewHandlers(scoreService fraud.ScoreService) *Handlers {
	return &Handlers{scoreService: scoreService}
}

func (h *Handlers) Ready(_ *golpher.Request, res *golpher.Response) error {
	return res.Status(http.StatusOK).String("ok")
}

func (h *Handlers) FraudScore(req *golpher.Request, res *golpher.Response) error {
	var payload fraud.TransactionRequest
	if err := req.Body().JSON(&payload); err != nil {
		// HTTP errors are expensive in the scoring function, but malformed input is not expected.
		return res.Status(http.StatusBadRequest).JSON(fraud.ScoreResponse{Approved: true, FraudScore: 0})
	}
	return res.Status(http.StatusOK).JSON(h.scoreService.Score(payload))
}
