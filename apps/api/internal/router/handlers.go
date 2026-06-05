package router

import (
	"io"
	"net/http"
	"sync"

	"github.com/whoisclebs/heindall/apps/api/internal/fraud"
)

type Handlers struct {
	scoreService *fraud.Service
	bodyLimit    int64
	bodyPool     sync.Pool
}

var scoreResponses = [...][]byte{
	[]byte(`{"approved":true,"fraud_score":0}`),
	[]byte(`{"approved":true,"fraud_score":0.2}`),
	[]byte(`{"approved":true,"fraud_score":0.4}`),
	[]byte(`{"approved":false,"fraud_score":0.6}`),
	[]byte(`{"approved":false,"fraud_score":0.8}`),
	[]byte(`{"approved":false,"fraud_score":1}`),
}

// Precomputed Content-Length header values to avoid per-request strconv.Itoa allocations.
var scoreContentLengths = [...]string{"33", "35", "35", "36", "36", "34"}

const (
	okContentType    = "text/plain; charset=utf-8"
	okContentLength  = "2"
	scoreContentType = "application/json"
)

func NewHandlers(scoreService *fraud.Service, bodyLimit int64) *Handlers {
	h := &Handlers{scoreService: scoreService, bodyLimit: bodyLimit}
	if bodyLimit > 0 && bodyLimit <= 1<<20 {
		size := int(bodyLimit)
		h.bodyPool.New = func() any {
			return make([]byte, size)
		}
	}
	return h
}

func (h *Handlers) ReadyRaw(w http.ResponseWriter, _ *http.Request) {
	header := w.Header()
	header.Set("Content-Type", okContentType)
	header.Set("Content-Length", okContentLength)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func (h *Handlers) FraudScoreRaw(w http.ResponseWriter, req *http.Request) {
	data, pooled, ok := h.readRequestBody(w, req)
	if !ok {
		return
	}
	if pooled != nil {
		defer h.releaseBodyBuffer(pooled)
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
	header := w.Header()
	header.Set("Content-Type", scoreContentType)
	header.Set("Content-Length", scoreContentLengths[frauds])
	w.WriteHeader(status)
	_, _ = w.Write(scoreResponses[frauds])
}

func (h *Handlers) readRequestBody(w http.ResponseWriter, req *http.Request) ([]byte, []byte, bool) {
	return readRequestBodyWithPool(w, req, h.bodyLimit, &h.bodyPool)
}

// readRequestBody preserves the test-facing helper signature while using the
// same implementation as the pooled handler path.
func readRequestBody(w http.ResponseWriter, req *http.Request, maxBytes int64) ([]byte, bool) {
	data, _, ok := readRequestBodyWithPool(w, req, maxBytes, nil)
	return data, ok
}

// readRequestBodyWithPool reads and validates the request body with awareness of
// Content-Length. When a pool is supplied and Content-Length is known, it reuses
// an exact-size slice backed by a pooled buffer to avoid per-request allocations.
func readRequestBodyWithPool(w http.ResponseWriter, req *http.Request, maxBytes int64, pool *sync.Pool) ([]byte, []byte, bool) {
	cl := req.ContentLength
	if maxBytes >= 0 && cl > maxBytes {
		writeScoreRaw(w, http.StatusRequestEntityTooLarge, 0)
		return nil, nil, false
	}
	if req.Body == nil {
		return nil, nil, true
	}

	// Fast path: known Content-Length, read exactly that many bytes.
	if cl >= 0 {
		if cl == 0 {
			return nil, nil, true
		}
		buf, pooled := acquireBodyBuffer(pool, cl)
		_, err := io.ReadFull(req.Body, buf)
		if err != nil {
			if pooled != nil {
				releaseBodyBuffer(pool, pooled)
			}
			writeScoreRaw(w, http.StatusBadRequest, 0)
			return nil, nil, false
		}
		return buf, pooled, true
	}

	// Slow path: unknown Content-Length (legacy / HTTP 1.0). Still bounded by maxBytes.
	if maxBytes < 0 {
		data, err := io.ReadAll(req.Body)
		if err != nil {
			writeScoreRaw(w, http.StatusInternalServerError, 0)
			return nil, nil, false
		}
		return data, nil, true
	}

	// Unknown Content-Length with a limit: use LimitReader to enforce the bound.
	limit := maxBytes + 1
	data, err := io.ReadAll(io.LimitReader(req.Body, limit))
	if err != nil {
		writeScoreRaw(w, http.StatusInternalServerError, 0)
		return nil, nil, false
	}
	if int64(len(data)) > maxBytes {
		writeScoreRaw(w, http.StatusRequestEntityTooLarge, 0)
		return nil, nil, false
	}
	return data, nil, true
}

func acquireBodyBuffer(pool *sync.Pool, size int64) ([]byte, []byte) {
	if size <= 0 {
		return nil, nil
	}
	if pool == nil {
		return make([]byte, size), nil
	}
	pooled, _ := pool.Get().([]byte)
	if int64(cap(pooled)) < size {
		return make([]byte, size), nil
	}
	return pooled[:size], pooled
}

func (h *Handlers) releaseBodyBuffer(pooled []byte) {
	releaseBodyBuffer(&h.bodyPool, pooled)
}

func releaseBodyBuffer(pool *sync.Pool, pooled []byte) {
	if pool == nil || pooled == nil {
		return
	}
	pool.Put(pooled[:cap(pooled)])
}
