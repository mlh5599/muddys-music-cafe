package api

import (
	"net"
	"net/http"
	"strings"
)

// hostRouter sends each request to the handler for its hostname, so the admin
// app and API exist only on the admin hostname and the public site only on the
// public one. Anything else gets a plain 404.
type hostRouter struct {
	public, admin http.Handler
	publicHost    []string
	adminHost     string
}

func (h hostRouter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	host := normalizeHost(r.Host)
	switch {
	case host == h.adminHost:
		h.admin.ServeHTTP(w, r)
	case contains(h.publicHost, host):
		h.public.ServeHTTP(w, r)
	default:
		http.NotFound(w, r)
	}
}

func normalizeHost(h string) string {
	if hh, _, err := net.SplitHostPort(h); err == nil {
		h = hh
	}
	return strings.ToLower(strings.TrimSuffix(h, "."))
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
