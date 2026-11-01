package api

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mlh5599/muddys-music-cafe/server/internal/storage"
)

func newTestMux(t *testing.T) *http.ServeMux {
	t.Helper()
	db, err := storage.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	mux := http.NewServeMux()
	if err := Register(mux, Deps{DB: db, UploadsDir: t.TempDir()}); err != nil {
		t.Fatal(err)
	}
	return mux
}

func TestHealthz(t *testing.T) {
	rec := httptest.NewRecorder()
	newTestMux(t).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := strings.TrimSpace(rec.Body.String()); got != "ok" {
		t.Errorf("body = %q, want ok", got)
	}
}

func TestSecurityHeaders(t *testing.T) {
	rec := httptest.NewRecorder()
	newTestMux(t).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	for _, h := range []string{"Content-Security-Policy", "X-Content-Type-Options", "X-Frame-Options", "Referrer-Policy"} {
		if rec.Header().Get(h) == "" {
			t.Errorf("missing header %s", h)
		}
	}
	if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q, want nosniff", got)
	}
}

func TestMetricsNotOnPublicMux(t *testing.T) {
	rec := httptest.NewRecorder()
	newTestMux(t).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if body := rec.Body.String(); strings.Contains(body, "http_requests_total") {
		t.Fatal("/metrics output is exposed on the public mux")
	}
}

func TestMetricsOnInternalMux(t *testing.T) {
	// Generate at least one observation first.
	newTestMux(t).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/healthz", nil))

	mux := http.NewServeMux()
	RegisterMetrics(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "http_requests_total") {
		t.Error("metrics output missing http_requests_total")
	}
}

func TestRecoverMiddleware(t *testing.T) {
	h := WithMiddleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("boom") }), recoverMiddleware)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
}
