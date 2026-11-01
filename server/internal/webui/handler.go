package webui

import (
	"io/fs"
	"net/http"
	"strings"
)

// Handler serves the embedded admin build (dist/) as static files, falling
// back to index.html for any path that is not a real file in the build so
// client-side routes work on a direct navigation or page refresh.
func Handler() http.Handler {
	sub, err := fs.Sub(DistFS, "dist")
	if err != nil {
		panic(err)
	}
	fileServer := http.FileServerFS(sub)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "."
		}
		if _, err := fs.Stat(sub, path); err != nil {
			clone := r.Clone(r.Context())
			clone.URL.Path = "/"
			fileServer.ServeHTTP(w, clone)
			return
		}
		fileServer.ServeHTTP(w, r)
	})
}
