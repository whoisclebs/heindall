package router

import (
	"io"
	"net/http"
	"strconv"

	"github.com/whoisclebs/heindall/apps/api/internal/fraud"
)

type Handlers struct {
	scoreService *fraud.Service
	bodyLimit    int64
}

var scoreResponses = [...][]byte{
	[]byte(`{"approved":true,"fraud_score":0}`),
	[]byte(`{"approved":true,"fraud_score":0.2}`),
	[]byte(`{"approved":true,"fraud_score":0.4}`),
	[]byte(`{"approved":false,"fraud_score":0.6}`),
	[]byte(`{"approved":false,"fraud_score":0.8}`),
	[]byte(`{"approved":false,"fraud_score":1}`),
}

func NewHandlers(scoreService *fraud.Service, bodyLimit int64) *Handlers {
	return &Handlers{scoreService: scoreService, bodyLimit: bodyLimit}
}

func (h *Handlers) ReadyRaw(w http.ResponseWriter, _ *http.Request) {
	writeBytes(w, http.StatusOK, "text/plain; charset=utf-8", []byte("ok"))
}

func (h *Handlers) FraudScoreRaw(w http.ResponseWriter, req *http.Request) {
	data, ok := readRequestBody(w, req, h.bodyLimit)
	if !ok {
		return
	}
	frauds, valid := h.scoreService.FraudCountJSON(data)
	if !valid {
		writeScoreRaw(w, http.StatusBadRequest, 0)
		return
	}
	writeScoreRaw(w, http.StatusOK, frauds)
}

func writeScoreRaw(w http.ResponseWriter, status int, frauds int) {
	if frauds < 0 {
		frauds = 0
	} else if frauds > 5 {
		frauds = 5
	}
	writeBytes(w, status, "application/json", scoreResponses[frauds])
}

func writeBytes(w http.ResponseWriter, status int, contentType string, body []byte) {
	header := w.Header()
	header.Set("Content-Type", contentType)
	header.Set("Content-Length", strconv.Itoa(len(body)))
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

func readRequestBody(w http.ResponseWriter, req *http.Request, maxBytes int64) ([]byte, bool) {
	if maxBytes >= 0 && req.ContentLength > maxBytes {
		writeScoreRaw(w, http.StatusRequestEntityTooLarge, 0)
		return nil, false
	}
	if req.Body == nil {
		return nil, true
	}
	if maxBytes < 0 {
		data, err := io.ReadAll(req.Body)
		if err != nil {
			writeScoreRaw(w, http.StatusInternalServerError, 0)
			return nil, false
		}
		return data, true
	}
	limit := maxBytes + 1
	data, err := io.ReadAll(io.LimitReader(req.Body, limit))
	if err != nil {
		writeScoreRaw(w, http.StatusInternalServerError, 0)
		return nil, false
	}
	if maxBytes >= 0 && int64(len(data)) > maxBytes {
		writeScoreRaw(w, http.StatusRequestEntityTooLarge, 0)
		return nil, false
	}
	return data, true
}
