package router

import (
	"bytes"
	"io"
	"net/http"
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

// Baseline: full raw handler path (read + score + write).
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

// Baseline: ready handler.
func BenchmarkReadyRawHandler(b *testing.B) {
	h := NewHandlers(benchmarkFraudService(b), 4096)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("GET", "/ready", nil)
		rw := httptest.NewRecorder()
		h.ReadyRaw(rw, req)
		if rw.Code != 200 {
			b.Fatalf("status = %d", rw.Code)
		}
	}
}

// Body read only, isolating the public helper path. The allocs reported here
// still include httptest request/recorder setup and are best used for relative
// comparisons, not as absolute hot-path allocation counts.
func BenchmarkReadRequestBodyExactSize(b *testing.B) {
	payload := benchmarkFraudPayload()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("POST", "/fraud-score", bytes.NewReader(payload))
		req.ContentLength = int64(len(payload))
		rw := httptest.NewRecorder()
		data, ok := readRequestBody(rw, req, 4096)
		if !ok || len(data) == 0 {
			b.Fatal("read failed")
		}
	}
}

type benchmarkBody struct {
	payload []byte
	offset  int
}

func (b *benchmarkBody) Reset(payload []byte) {
	b.payload = payload
	b.offset = 0
}

func (b *benchmarkBody) Read(p []byte) (int, error) {
	if b.offset >= len(b.payload) {
		return 0, io.EOF
	}
	n := copy(p, b.payload[b.offset:])
	b.offset += n
	return n, nil
}

func (b *benchmarkBody) Close() error { return nil }

type benchmarkResponseWriter struct {
	header http.Header
	code   int
}

func (w *benchmarkResponseWriter) Header() http.Header {
	if w.header == nil {
		w.header = make(http.Header)
	}
	return w.header
}

func (w *benchmarkResponseWriter) Write(p []byte) (int, error) { return len(p), nil }

func (w *benchmarkResponseWriter) WriteHeader(statusCode int) { w.code = statusCode }

// Focused fast-path benchmark without httptest noise. This measures the pooled
// Content-Length-aware body read as it is exercised from the real handler.
func BenchmarkReadRequestBodyPooledFastPath(b *testing.B) {
	h := NewHandlers(benchmarkFraudService(b), 4096)
	payload := benchmarkFraudPayload()
	body := &benchmarkBody{}
	req := &http.Request{Body: body, ContentLength: int64(len(payload))}
	rw := &benchmarkResponseWriter{}

	// Pre-warm the pool so per-iteration alloc counts reflect steady-state usage.
	body.Reset(payload)
	data, pooled, ok := h.readRequestBody(rw, req)
	if !ok || len(data) == 0 {
		b.Fatal("warmup read failed")
	}
	if pooled != nil {
		h.releaseBodyBuffer(pooled)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		body.Reset(payload)
		req.ContentLength = int64(len(payload))
		rw.code = 0
		data, pooled, ok := h.readRequestBody(rw, req)
		if !ok || len(data) == 0 {
			b.Fatal("read failed")
		}
		if pooled != nil {
			h.releaseBodyBuffer(pooled)
		}
	}
}

// Combined: writeScoreRaw path only (body already read).
func BenchmarkWriteScoreRaw(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rw := httptest.NewRecorder()
		writeScoreRaw(rw, 200, 2)
		_ = rw.Body.Bytes()
		// Prevent write from being optimized away.
		if len(rw.Body.Bytes()) == 0 {
			b.Fatal("empty response")
		}
	}
}

// Body read with unknown Content-Length, which forces the slower fallback path.
func BenchmarkReadRequestBodyUnknownLength(b *testing.B) {
	payload := benchmarkFraudPayload()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("POST", "/fraud-score", bytes.NewReader(payload))
		req.ContentLength = -1
		rw := httptest.NewRecorder()
		data, ok := readRequestBody(rw, req, 4096)
		if !ok || len(data) == 0 {
			b.Fatal("read failed")
		}
	}
}

// Handler path with unknown Content-Length (exercises the LimitReader fallback).
func BenchmarkFraudScoreRawHandlerUnknownLength(b *testing.B) {
	h := NewHandlers(benchmarkFraudService(b), 4096)
	payload := benchmarkFraudPayload()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("POST", "/fraud-score", bytes.NewReader(payload))
		req.ContentLength = -1
		rw := httptest.NewRecorder()
		h.FraudScoreRaw(rw, req)
		if rw.Code != 200 {
			b.Fatalf("status = %d", rw.Code)
		}
	}
}
