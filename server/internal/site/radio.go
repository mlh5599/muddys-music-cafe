package site

import (
	"context"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/mlh5599/muddys-music-cafe/server/internal/storage"
)

// radioPath is where the site's player finds the radio. The stream host only
// speaks plain http, which browsers block on an https page, so the site relays
// the audio from its own (https) origin.
const radioPath = "/radio/stream"

// maxListeners caps simultaneous relayed listeners; each one holds an upstream
// connection and the stream host has its own listener limit.
const maxListeners = 200

var radioClient = &http.Client{Transport: &http.Transport{
	ResponseHeaderTimeout: 10 * time.Second,
	DisableCompression:    true,
}}

// serveRadio relays the configured stream. The upstream address comes only
// from the admin-controlled setting, never from the request.
func (s *Site) serveRadio(w http.ResponseWriter, r *http.Request) {
	upstream, err := s.settings.Get(r.Context(), storage.SettingRadioStreamURL)
	upstream = strings.TrimSpace(upstream)
	if err != nil || upstream == "" {
		http.Error(w, "the radio is offline", http.StatusServiceUnavailable)
		return
	}
	select {
	case s.listeners <- struct{}{}:
		defer func() { <-s.listeners }()
	default:
		http.Error(w, "the radio is busy, try again shortly", http.StatusServiceUnavailable)
		return
	}

	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, upstream, nil)
	if err != nil {
		http.Error(w, "the radio is offline", http.StatusServiceUnavailable)
		return
	}
	req.Header.Set("User-Agent", "muddys-site-radio-relay")
	resp, err := radioClient.Do(req)
	if err != nil {
		if r.Context().Err() == nil {
			log.Printf("radio relay: upstream: %v", err)
		}
		http.Error(w, "the radio is unavailable", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		http.Error(w, "the radio is unavailable", http.StatusBadGateway)
		return
	}

	ct := resp.Header.Get("Content-Type")
	if ct == "" {
		ct = "audio/mpeg"
	}
	h := w.Header()
	h.Set("Content-Type", ct)
	h.Set("Cache-Control", "no-store")
	h.Set("X-Accel-Buffering", "no") // don't let a proxy buffer a live stream
	w.WriteHeader(http.StatusOK)

	rc := http.NewResponseController(w)
	_ = rc.SetWriteDeadline(time.Time{}) // the server's write timeout would cut a live stream
	buf := make([]byte, 16<<10)
	for {
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := w.Write(buf[:n]); werr != nil {
				return
			}
			_ = rc.Flush()
		}
		if rerr != nil {
			if rerr != io.EOF && r.Context().Err() == nil && rerr != context.Canceled {
				log.Printf("radio relay: read: %v", rerr)
			}
			return
		}
	}
}
