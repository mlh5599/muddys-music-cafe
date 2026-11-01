package webui

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandler_FallsBackToIndexForClientRoutes(t *testing.T) {
	h := Handler()

	rootRec := httptest.NewRecorder()
	h.ServeHTTP(rootRec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rootRec.Code != http.StatusOK {
		t.Fatalf("GET / status = %d, want 200", rootRec.Code)
	}

	routeRec := httptest.NewRecorder()
	h.ServeHTTP(routeRec, httptest.NewRequest(http.MethodGet, "/events", nil))
	if routeRec.Code != http.StatusOK {
		t.Fatalf("GET /events status = %d, want 200 (SPA fallback)", routeRec.Code)
	}
	if routeRec.Body.String() != rootRec.Body.String() {
		t.Errorf("GET /events body differs from GET /; expected the same index.html fallback")
	}
}
