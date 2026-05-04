package router

import (
	"net/http"

	"github.com/go-golpher/golpher"
	"github.com/whoisclebs/heindall/apps/api/internal/fraud"
)

type Handlers struct {
	scoreService *fraud.Service
}

var scoreResponses = [...][]byte{
	[]byte(`{"approved":true,"fraud_score":0}`),
	[]byte(`{"approved":true,"fraud_score":0.2}`),
	[]byte(`{"approved":true,"fraud_score":0.4}`),
	[]byte(`{"approved":false,"fraud_score":0.6}`),
	[]byte(`{"approved":false,"fraud_score":0.8}`),
	[]byte(`{"approved":false,"fraud_score":1}`),
}

func NewHandlers(scoreService *fraud.Service) *Handlers {
	return &Handlers{scoreService: scoreService}
}

func (h *Handlers) Ready(_ *golpher.Request, res *golpher.Response) error {
	return res.Status(http.StatusOK).String("ok")
}

func (h *Handlers) FraudScore(req *golpher.Request, res *golpher.Response) error {
	frauds, valid := h.scoreService.FraudCountJSON(req.Body().Bytes())
	if !valid {
		return writeScoreResponseStatus(res, http.StatusBadRequest, 0)
	}
	return writeScoreResponse(res, frauds)
}

func writeScoreResponse(res *golpher.Response, frauds int) error {
	if frauds < 0 {
		frauds = 0
	} else if frauds > 5 {
		frauds = 5
	}
	res.Raw().Header().Set("Content-Type", "application/json")
	return res.Status(http.StatusOK).Send(scoreResponses[frauds])
}

func writeScoreResponseStatus(res *golpher.Response, status int, frauds int) error {
	if frauds < 0 {
		frauds = 0
	} else if frauds > 5 {
		frauds = 5
	}
	res.Raw().Header().Set("Content-Type", "application/json")
	return res.Status(status).Send(scoreResponses[frauds])
}
