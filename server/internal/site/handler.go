// Package site serves the public website pages. In Phase 1 it is a
// placeholder; the real templates arrive in Phase 4.
package site

import (
	"net/http"
)

// Handler returns the public site handler.
func Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte("<!doctype html><title>Muddy's Music Cafe</title><p>Coming soon.</p>\n"))
	})
}
