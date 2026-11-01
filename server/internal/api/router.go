// Package api wires HTTP routes for the website.
package api

import (
	"database/sql"
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/mlh5599/muddys-music-cafe/server/internal/site"
	"github.com/mlh5599/muddys-music-cafe/server/internal/webui"
)

// Deps holds the dependencies Register wires into route handlers. It grows as
// storage and uploads land in later phases.
type Deps struct {
	DB         *sql.DB
	UploadsDir string
}

// Register attaches all public-facing routes to mux, each wrapped in the
// standard middleware chain. It deliberately does NOT register /metrics:
// that is served on a separate internal-only listener (see RegisterMetrics)
// so it is never reachable through the public hostname.
func Register(mux *http.ServeMux, d Deps) error {
	siteHandler, err := site.New(site.Config{DB: d.DB, UploadsDir: d.UploadsDir})
	if err != nil {
		return err
	}
	mux.Handle("GET /healthz", withStandardMiddleware(http.HandlerFunc(healthz)))
	mux.Handle("/admin/", withStandardMiddleware(http.StripPrefix("/admin", webui.Handler())))
	mux.Handle("/", withStandardMiddleware(siteHandler))
	return nil
}

// RegisterMetrics attaches /metrics to a mux served on the internal-only
// metrics listener.
func RegisterMetrics(mux *http.ServeMux) {
	mux.Handle("GET /metrics", WithMiddleware(promhttp.Handler(), recoverMiddleware, loggingMiddleware))
}

func healthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte("ok\n"))
}

func withStandardMiddleware(h http.Handler) http.Handler {
	return WithMiddleware(h, recoverMiddleware, securityHeadersMiddleware, loggingMiddleware, metricsMiddleware)
}
