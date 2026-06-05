package router

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/whoisclebs/heindall/apps/api/internal/fraud"
)

// --- Helpers ---

func testFraudService(t *testing.T) *fraud.Service {
	t.Helper()
	refs := make([]fraud.Reference, 200)
	for i := range refs {
		v := float32(i%100) / 100
		refs[i] = fraud.Reference{Vector: [fraud.Dimensions]float32{v, 0.25, 0.5, 0.75, 0.1, -1, -1, v, 0.2, 1, 0, 1, 0.75, 0.01}, Label: fraud.LabelLegit}
		if i%7 == 0 {
			refs[i].Label = fraud.LabelFraud
		}
	}
	return fraud.NewService(fraud.DefaultNormalization(), fraud.DefaultMCCRisk(), fraud.NewExactSearcher(refs))
}

func testHandlers(t *testing.T, bodyLimit int64) *Handlers {
	t.Helper()
	return NewHandlers(testFraudService(t), bodyLimit)
}

// --- 1.2 Correctness fixtures ---

const knownPayload = `{"id":"tx-1329056812","transaction":{"amount":2575.75,"installments":7,"requested_at":"2026-03-27T09:08:20Z"},"customer":{"avg_amount":217.64,"tx_count_24h":7,"known_merchants":["MERC-005","MERC-003","MERC-001","MERC-011"]},"merchant":{"id":"MERC-003","mcc":"7802","avg_amount":293.16},"terminal":{"is_online":true,"card_present":false,"km_from_home":397.3207551537},"last_transaction":{"timestamp":"2026-03-27T07:53:20Z","km_from_current":51.1775721868}}`

func TestFraudScoreRawReturns200ForKnownPayload(t *testing.T) {
	h := testHandlers(t, 4096)

	req := httptest.NewRequest(http.MethodPost, "/fraud-score", bytes.NewReader([]byte(knownPayload)))
	req.ContentLength = int64(len(knownPayload))
	rw := httptest.NewRecorder()
	h.FraudScoreRaw(rw, req)

	if rw.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rw.Code)
	}
}

func TestFraudScoreRawReturnsAppropriateJSON(t *testing.T) {
	h := testHandlers(t, 4096)

	req := httptest.NewRequest(http.MethodPost, "/fraud-score", bytes.NewReader([]byte(knownPayload)))
	req.ContentLength = int64(len(knownPayload))
	rw := httptest.NewRecorder()
	h.FraudScoreRaw(rw, req)

	if rw.Code != http.StatusOK {
		t.Fatalf("status = %d", rw.Code)
	}
	contentType := rw.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", contentType)
	}
	if rw.Body.Len() == 0 {
		t.Fatal("response body is empty")
	}
	body := rw.Body.String()
	if !bytes.HasPrefix([]byte(body), []byte(`{"approved"`)) {
		t.Fatalf("unexpected body: %s", body)
	}
}

func TestReadyRawReturns200Ok(t *testing.T) {
	h := testHandlers(t, 4096)

	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	rw := httptest.NewRecorder()
	h.ReadyRaw(rw, req)

	if rw.Code != http.StatusOK {
		t.Fatalf("status = %d", rw.Code)
	}
	if rw.Body.String() != "ok" {
		t.Fatalf("body = %q, want ok", rw.Body.String())
	}
	if rw.Header().Get("Content-Type") != "text/plain; charset=utf-8" {
		t.Fatalf("Content-Type = %q", rw.Header().Get("Content-Type"))
	}
	if rw.Header().Get("Content-Length") != "2" {
		t.Fatalf("Content-Length = %q, want 2", rw.Header().Get("Content-Length"))
	}
}

func TestFraudScoreRawDetectsEmptyBody(t *testing.T) {
	h := testHandlers(t, 4096)

	req := httptest.NewRequest(http.MethodPost, "/fraud-score", bytes.NewReader(nil))
	req.ContentLength = 0
	rw := httptest.NewRecorder()
	h.FraudScoreRaw(rw, req)

	if rw.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rw.Code)
	}
}

// --- 2.1 Exact-size body reads ---

func TestReadRequestBodyExactSize(t *testing.T) {
	payload := []byte(knownPayload)
	req := httptest.NewRequest(http.MethodPost, "/fraud-score", bytes.NewReader(payload))
	req.ContentLength = int64(len(payload))
	rw := httptest.NewRecorder()

	data, ok := readRequestBody(rw, req, 4096)
	if !ok {
		t.Fatal("readRequestBody returned false")
	}
	if !bytes.Equal(data, payload) {
		t.Fatalf("data mismatch: got %d bytes, want %d bytes", len(data), len(payload))
	}
	// httptest.ResponseRecorder defaults Code to 200, but no WriteHeader was called
	// from readRequestBody on success. Check body is unchanged.
	if rw.Body.Len() != 0 {
		t.Fatalf("unexpected response body written: %q", rw.Body.String())
	}
}

func TestReadRequestBodyExactSizeZeroContentLength(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/fraud-score", nil)
	req.ContentLength = 0
	rw := httptest.NewRecorder()

	data, ok := readRequestBody(rw, req, 4096)
	if !ok {
		t.Fatal("readRequestBody returned false")
	}
	if data != nil {
		t.Fatal("expected nil data for zero Content-Length")
	}
}

// --- 2.1 Oversize payload rejection ---

func TestReadRequestBodyRejectsOversizeContentLength(t *testing.T) {
	payload := []byte(knownPayload)
	req := httptest.NewRequest(http.MethodPost, "/fraud-score", bytes.NewReader(payload))
	// Declare Content-Length that exceeds the body limit.
	req.ContentLength = 4096 + 1
	rw := httptest.NewRecorder()

	_, ok := readRequestBody(rw, req, 4096)
	if ok {
		t.Fatal("readRequestBody returned true for oversize Content-Length")
	}
	if rw.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d", rw.Code, http.StatusRequestEntityTooLarge)
	}
}

func TestReadRequestBodyRejectsOversizePayloadAfterRead(t *testing.T) {
	bigPayload := bytes.Repeat([]byte("x"), 5000)
	req := httptest.NewRequest(http.MethodPost, "/fraud-score", bytes.NewReader(bigPayload))
	// Declare a Content-Length that is within limits, but the body is bigger.
	// io.ReadFull will read exactly Content-Length bytes, then the extra Read
	// will find more data, triggering the oversize detection.
	req.ContentLength = 4000
	rw := httptest.NewRecorder()

	_, ok := readRequestBody(rw, req, 4096)
	if ok {
		t.Fatal("readRequestBody returned true for oversized body")
	}
	// Should detect extra bytes
	if rw.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rw.Code)
	}
}

func TestReadRequestBodyRejectsUnknownLengthOversize(t *testing.T) {
	bigPayload := bytes.Repeat([]byte("x"), 5000)
	req := httptest.NewRequest(http.MethodPost, "/fraud-score", bytes.NewReader(bigPayload))
	req.ContentLength = -1
	rw := httptest.NewRecorder()

	_, ok := readRequestBody(rw, req, 4096)
	if ok {
		t.Fatal("readRequestBody returned true for oversized body (unknown length)")
	}
	if rw.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d", rw.Code, http.StatusRequestEntityTooLarge)
	}
}

// --- 2.1 Malformed/truncated Content-Length handling ---

type truncatedReader struct {
	data       []byte
	pos        int
	truncateAt int
}

func (r *truncatedReader) Read(p []byte) (int, error) {
	if r.pos >= r.truncateAt {
		return 0, io.EOF
	}
	limit := r.truncateAt - r.pos
	if limit > len(p) {
		limit = len(p)
	}
	n := copy(p, r.data[r.pos:r.pos+limit])
	r.pos += n
	return n, nil
}

func (r *truncatedReader) Close() error { return nil }

func TestReadRequestBodyDetectsTruncatedBody(t *testing.T) {
	payload := []byte(knownPayload)
	req := httptest.NewRequest(http.MethodPost, "/fraud-score", nil)
	req.ContentLength = int64(len(payload))
	// truncatedReader reports EOF after 50 bytes total, but Content-Length is 578.
	req.Body = io.NopCloser(&truncatedReader{data: payload, truncateAt: 50})
	rw := httptest.NewRecorder()

	_, ok := readRequestBody(rw, req, 4096)
	if ok {
		t.Fatal("readRequestBody accepted truncated body")
	}
	if rw.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rw.Code)
	}
}

func TestReadRequestBodyHandlesZeroContentLengthWithBody(t *testing.T) {
	payload := []byte(knownPayload)
	req := httptest.NewRequest(http.MethodPost, "/fraud-score", bytes.NewReader(payload))
	req.ContentLength = 0
	rw := httptest.NewRecorder()

	data, ok := readRequestBody(rw, req, 4096)
	if !ok {
		t.Fatal("readRequestBody returned false for Content-Length 0")
	}
	// Content-Length 0 means we return nil (empty body)
	if data != nil {
		t.Fatalf("expected nil data for zero Content-Length, got %d bytes", len(data))
	}
}

func TestReadRequestBodyHandlesNegativeContentLength(t *testing.T) {
	payload := []byte(knownPayload)
	req := httptest.NewRequest(http.MethodPost, "/fraud-score", bytes.NewReader(payload))
	req.ContentLength = -1
	rw := httptest.NewRecorder()

	data, ok := readRequestBody(rw, req, 4096)
	if !ok {
		t.Fatal("readRequestBody returned false for unknown Content-Length")
	}
	if !bytes.Equal(data, payload) {
		t.Fatalf("data mismatch for unknown Content-Length")
	}
}

func TestReadRequestBodyContentLengthGTEZeroNilBody(t *testing.T) {
	// req.Body is nil but Content-Length is set to 0.
	// Should be handled by the nil-body check.
	req := httptest.NewRequest(http.MethodPost, "/fraud-score", nil)
	req.ContentLength = 0
	req.Body = nil
	rw := httptest.NewRecorder()

	data, ok := readRequestBody(rw, req, 4096)
	if !ok {
		t.Fatal("readRequestBody returned false for nil body")
	}
	if data != nil {
		t.Fatalf("expected nil data for nil body, got %d bytes", len(data))
	}
}

// --- 2.1 Repeated handler usage / buffer reuse ---

func TestHandlerCanBeCalledRepeatedly(t *testing.T) {
	h := testHandlers(t, 4096)
	payload := []byte(knownPayload)

	for i := 0; i < 100; i++ {
		req := httptest.NewRequest(http.MethodPost, "/fraud-score", bytes.NewReader(payload))
		req.ContentLength = int64(len(payload))
		rw := httptest.NewRecorder()
		h.FraudScoreRaw(rw, req)
		if rw.Code != http.StatusOK {
			t.Fatalf("iteration %d: status = %d", i, rw.Code)
		}
	}
}

func TestReadyCanBeCalledRepeatedly(t *testing.T) {
	h := testHandlers(t, 4096)

	for i := 0; i < 100; i++ {
		req := httptest.NewRequest(http.MethodGet, "/ready", nil)
		rw := httptest.NewRecorder()
		h.ReadyRaw(rw, req)
		if rw.Code != http.StatusOK {
			t.Fatalf("iteration %d: status = %d", i, rw.Code)
		}
		if rw.Body.String() != "ok" {
			t.Fatalf("iteration %d: body mismatch: got %q", i, rw.Body.String())
		}
	}
}

func TestWriteScoreRawClampsFrauds(t *testing.T) {
	tests := []struct {
		frauds  int
		wantLen string
	}{
		{-1, "33"},
		{0, "33"},
		{1, "35"},
		{2, "35"},
		{3, "36"},
		{4, "36"},
		{5, "34"},
		{6, "34"},
		{100, "34"},
	}

	for _, tt := range tests {
		rw := httptest.NewRecorder()
		writeScoreRaw(rw, http.StatusOK, tt.frauds)
		if rw.Header().Get("Content-Length") != tt.wantLen {
			t.Errorf("frauds=%d: Content-Length = %q, want %q", tt.frauds, rw.Header().Get("Content-Length"), tt.wantLen)
		}
	}
}

func TestScoreContentLengthsMatchBodies(t *testing.T) {
	for i, body := range scoreResponses {
		expected := scoreContentLengths[i]
		actual := itoa(len(body))
		if actual != expected {
			t.Errorf("scoreResponses[%d] len=%d, expected Content-Length=%q, got precomputed=%q", i, len(body), actual, expected)
		}
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
