// Command deployhook receives signed deploy requests from CI and runs the
// deploy script. It listens on a local address; a tunnel or proxy in front of
// it provides the public, access-controlled endpoint.
package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/mlh5599/muddys-music-cafe/server/internal/deployhook"
)

func main() {
	listen := flag.String("listen", "127.0.0.1:9080", "listen address")
	secretFile := flag.String("secret-file", "", "file containing the shared HMAC secret")
	script := flag.String("script", "", "deploy script, run as: script TAG DIGEST")
	timeout := flag.Duration("timeout", 10*time.Minute, "maximum time for one deploy")
	flag.Parse()

	if *secretFile == "" || *script == "" {
		log.Fatal("-secret-file and -script are required")
	}
	secret, err := os.ReadFile(*secretFile)
	if err != nil {
		log.Fatalf("read secret: %v", err)
	}
	secret = []byte(strings.TrimSpace(string(secret)))
	if len(secret) < 32 {
		log.Fatal("secret must be at least 32 characters")
	}

	srv := &http.Server{
		Addr:              *listen,
		Handler:           deployhook.New(secret, deployhook.ScriptRunner(*script), *timeout),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 14,
	}
	log.Printf("deployhook listening on %s", *listen)
	log.Fatal(srv.ListenAndServe())
}
