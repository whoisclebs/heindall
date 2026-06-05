package app

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-golpher/golpher"
)

func TestRegisterPprofServesIndexAndProfiles(t *testing.T) {
	app := golpher.New(golpher.AppConfig{DisableBanner: true})
	registerPprof(app)

	tests := []struct {
		path string
		want string
	}{
		{path: "/debug/pprof/", want: "profile"},
		{path: "/debug/pprof/goroutine?debug=1", want: "goroutine profile"},
	}

	for _, tt := range tests {
		req := httptest.NewRequest(http.MethodGet, tt.path, nil)
		rw := httptest.NewRecorder()
		app.ServeHTTP(rw, req)
		if rw.Code != http.StatusOK {
			t.Fatalf("%s: status = %d, want %d", tt.path, rw.Code, http.StatusOK)
		}
		if !strings.Contains(rw.Body.String(), tt.want) {
			t.Fatalf("%s: body does not contain %q", tt.path, tt.want)
		}
	}
}
