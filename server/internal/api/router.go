// Package api wires HTTP routes for the website.
package api

import (
	"database/sql"
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/mlh5599/muddys-music-cafe/server/internal/admin"
	"github.com/mlh5599/muddys-music-cafe/server/internal/site"
	"github.com/mlh5599/muddys-music-cafe/server/internal/webui"
)

// Deps holds the dependencies Register wires into route handlers.
type Deps struct {
	DB         *sql.DB
	UploadsDir string
	Verifier   *admin.Verifier // access-gate assertion checker; nil locks the admin API
	DevNoAuth  bool            // development only: skip the assertion check

	// Hostnames. With both PublicHost and AdminHost set, the admin app and API
	// are served only on AdminHost and the public site only on PublicHost (and
	// ExtraPublicHosts, e.g. www). With either empty, every route is served on
	// every hostname (local development).
	PublicHost       string
	ExtraPublicHosts []string
	AdminHost        string
	SiteURL          string // public origin for absolute link-preview URLs
}

// Register attaches all public-facing routes to mux, each wrapped in the
// standard middleware chain. It deliberately does NOT register /metrics:
// that is served on a separate internal-only listener (see RegisterMetrics)
// so it is never reachable through the public hostname.
func Register(mux *http.ServeMux, d Deps) error {
	siteHandler, err := site.New(site.Config{DB: d.DB, UploadsDir: d.UploadsDir, SiteURL: d.SiteURL})
	if err != nil {
		return err
	}
	adminAPI := withStandardMiddleware(admin.New(admin.Config{
		DB: d.DB, UploadsDir: d.UploadsDir, Verifier: d.Verifier, DevNoAuth: d.DevNoAuth,
	}))
	adminUI := withStandardMiddleware(http.StripPrefix("/admin", webui.Handler()))
	public := withStandardMiddleware(siteHandler)
	health := withStandardMiddleware(http.HandlerFunc(healthz))

	if d.AdminHost == "" || d.PublicHost == "" {
		// Single-host mode (local development): everything on one hostname.
		mux.Handle("GET /healthz", health)
		mux.Handle("/api/admin/", adminAPI)
		mux.Handle("/admin/", adminUI)
		mux.Handle("/", public)
		return nil
	}

	// Split mode: the admin app and API exist only on the admin hostname.
	pub := http.NewServeMux()
	pub.Handle("GET /healthz", health)
	pub.Handle("/", public)

	adm := http.NewServeMux()
	adm.Handle("GET /healthz", health)
	adm.Handle("/api/admin/", adminAPI)
	adm.Handle("/admin/", adminUI)
	adm.Handle("GET /{$}", http.RedirectHandler("/admin/", http.StatusFound))

	hosts := []string{normalizeHost(d.PublicHost)}
	for _, a := range d.ExtraPublicHosts {
		hosts = append(hosts, normalizeHost(a))
	}
	mux.Handle("/", hostRouter{public: pub, admin: adm, publicHost: hosts, adminHost: normalizeHost(d.AdminHost)})
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
