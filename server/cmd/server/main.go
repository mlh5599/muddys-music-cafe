// Command server runs the Muddy's Music Cafe website: the public pages, the
// admin API and the embedded admin app, plus an internal-only metrics listener.
package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/mlh5599/muddys-music-cafe/server/internal/admin"
	"github.com/mlh5599/muddys-music-cafe/server/internal/api"
	"github.com/mlh5599/muddys-music-cafe/server/internal/seed"
	"github.com/mlh5599/muddys-music-cafe/server/internal/storage"
)

func main() {
	addr := flag.String("addr", ":8080", "listen address")
	metricsAddr := flag.String("metrics-addr", ":9090", "internal-only listen address for /metrics")
	dataDir := flag.String("data-dir", "./data", "directory for persistent data (SQLite database, uploads)")
	accessTeam := flag.String("access-team-domain", "", "access gate team domain; with -access-aud, enables the admin API")
	accessAud := flag.String("access-aud", "", "access gate application audience tag")
	devNoAuth := flag.Bool("dev-no-auth", false, "DEVELOPMENT ONLY: skip admin sign-in checks (loopback addresses only)")
	publicHost := flag.String("public-host", "", "public site hostname; with -admin-host, serves the public site only on it")
	extraHosts := flag.String("public-host-alias", "", "comma-separated extra hostnames that serve the public site (e.g. www)")
	adminHost := flag.String("admin-host", "", "admin hostname; the admin app and API are served only on it")
	siteURL := flag.String("site-url", "", "public origin, e.g. https://example.com (makes link-preview image URLs absolute)")
	seedFlag := flag.Bool("seed", false, "import the starter content into an empty database, then exit")
	flag.Parse()

	if err := os.MkdirAll(*dataDir, 0o750); err != nil {
		log.Fatalf("create data dir %s: %v", *dataDir, err)
	}

	mux := http.NewServeMux()
	db, err := storage.Open(filepath.Join(*dataDir, "muddys.db"))
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer db.Close()

	if *seedFlag {
		if err := seed.Run(context.Background(), db, filepath.Join(*dataDir, "uploads")); err != nil {
			log.Fatalf("seed: %v", err)
		}
		log.Print("seed: starter content imported")
		return
	}

	if (*publicHost == "") != (*adminHost == "") {
		log.Fatal("-public-host and -admin-host must be set together")
	}
	var verifier *admin.Verifier
	switch {
	case *devNoAuth:
		if !isLoopback(*addr) {
			log.Fatal("-dev-no-auth is only allowed when -addr is a loopback address (e.g. 127.0.0.1:8080)")
		}
		log.Print("WARNING: admin sign-in checks are disabled (-dev-no-auth)")
	case *accessTeam != "" && *accessAud != "":
		verifier, err = admin.NewVerifier(*accessTeam, *accessAud, nil)
		if err != nil {
			log.Fatalf("access gate config: %v", err)
		}
	default:
		log.Print("admin API is locked: set -access-team-domain and -access-aud to enable it")
	}

	if err := api.Register(mux, api.Deps{
		DB: db, UploadsDir: filepath.Join(*dataDir, "uploads"),
		Verifier: verifier, DevNoAuth: *devNoAuth,
		PublicHost: *publicHost, AdminHost: *adminHost, SiteURL: *siteURL,
		ExtraPublicHosts: splitList(*extraHosts),
	}); err != nil {
		log.Fatalf("register routes: %v", err)
	}

	metricsMux := http.NewServeMux()
	api.RegisterMetrics(metricsMux)

	srv := newServer(*addr, mux)
	metricsSrv := newServer(*metricsAddr, metricsMux)

	errc := make(chan error, 2)
	go func() {
		log.Printf("metrics listening on %s", *metricsAddr)
		errc <- metricsSrv.ListenAndServe()
	}()
	go func() {
		log.Printf("listening on %s", *addr)
		errc <- srv.ListenAndServe()
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errc:
		if !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server error: %v", err)
		}
	case sig := <-stop:
		log.Printf("received %s, shutting down", sig)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
	_ = metricsSrv.Shutdown(ctx)
}

// newServer returns an http.Server with timeouts set, so slow or stalled
// clients cannot hold connections open indefinitely.
func newServer(addr string, h http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           h,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 16,
	}
}

// isLoopback reports whether addr binds only to the local machine.
func isLoopback(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil || host == "" {
		return false
	}
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func splitList(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
