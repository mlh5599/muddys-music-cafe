// Command server runs the Muddy's Music Cafe website: the public pages, the
// admin API and the embedded admin app, plus an internal-only metrics listener.
package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mlh5599/muddys-music-cafe/server/internal/api"
)

func main() {
	addr := flag.String("addr", ":8080", "listen address")
	metricsAddr := flag.String("metrics-addr", ":9090", "internal-only listen address for /metrics")
	dataDir := flag.String("data-dir", "./data", "directory for persistent data (SQLite database, uploads)")
	flag.Parse()

	if err := os.MkdirAll(*dataDir, 0o750); err != nil {
		log.Fatalf("create data dir %s: %v", *dataDir, err)
	}

	mux := http.NewServeMux()
	api.Register(mux, api.Deps{})

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
