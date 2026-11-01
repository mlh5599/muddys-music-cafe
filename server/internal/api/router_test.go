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

func TestAdminAPILockedByDefault(t *testing.T) {
	mux := newTestMux(t)
	for _, p := range []string{"/api/admin/events", "/api/admin/me"} {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, p, nil))
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s status = %d, want 401", p, rec.Code)
		}
	}
}

func TestHostSplit(t *testing.T) {
	db, err := storage.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	mux := http.NewServeMux()
	if err := Register(mux, Deps{
		DB: db, UploadsDir: t.TempDir(), DevNoAuth: true,
		PublicHost: "site.example.test", ExtraPublicHosts: []string{"www.site.example.test"},
		AdminHost: "admin.example.test", SiteURL: "https://site.example.test",
	}); err != nil {
		t.Fatal(err)
	}
	get := func(host, path string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodGet, path, nil)
		r.Host = host
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, r)
		return rec
	}
	cases := []struct {
		host, path string
		want       int
	}{
		{"site.example.test", "/", 200},
		{"www.site.example.test", "/djs", 200},
		{"SITE.example.test:443", "/", 200},
		{"site.example.test", "/healthz", 200},
		{"site.example.test", "/admin/", 404},
		{"site.example.test", "/api/admin/events", 404},
		{"admin.example.test", "/api/admin/events", 200},
		{"admin.example.test", "/admin/", 200},
		{"admin.example.test", "/", 302},
		{"admin.example.test", "/djs", 404},
		{"admin.example.test", "/healthz", 200},
		{"evil.test", "/", 404},
		{"evil.test", "/api/admin/events", 404},
	}
	for _, c := range cases {
		if got := get(c.host, c.path).Code; got != c.want {
			t.Errorf("%s%s = %d, want %d", c.host, c.path, got, c.want)
		}
	}
	if body := get("site.example.test", "/").Body.String(); !strings.Contains(body, `content="https://site.example.test/static/images/logo.png"`) {
		t.Error("og:image is not absolute")
	}
}
