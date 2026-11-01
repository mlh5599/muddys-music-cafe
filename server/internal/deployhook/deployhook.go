// Package deployhook is the receiver that lets CI trigger a deploy over HTTP.
//
// It is deliberately dumb: it authenticates a request, validates that it names
// a release tag and an image digest (and nothing else), and runs one fixed
// script with those two values. It never takes a command, path or image
// repository from the request.
package deployhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os/exec"
	"regexp"
	"strconv"
	"sync"
	"time"
)

var (
	tagRe    = regexp.MustCompile(`^v\d{4}\.\d{2}\.\d{2}\.\d+$`)
	digestRe = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
	idRe     = regexp.MustCompile(`^[0-9a-f]{32}$`)
)

const (
	// MaxSkew bounds how far a request's timestamp may be from our clock, which
	// limits how long a captured request could be replayed.
	MaxSkew     = 5 * time.Minute
	maxBody     = 4 << 10
	maxLogBytes = 64 << 10
)

// Sign returns the hex HMAC-SHA256 of msg, the value CI sends in X-Signature.
func Sign(secret []byte, msg string) string {
	m := hmac.New(sha256.New, secret)
	m.Write([]byte(msg))
	return hex.EncodeToString(m.Sum(nil))
}

// Request is the only thing a deploy request may carry.
type Request struct {
	Tag    string `json:"tag"`
	Digest string `json:"digest"`
}

// State of a deploy.
const (
	StateRunning   = "running"
	StateSucceeded = "succeeded"
	StateFailed    = "failed"
)

// Status is what GET /deploy/{id} returns.
type Status struct {
	ID     string `json:"id"`
	Tag    string `json:"tag"`
	Digest string `json:"digest"`
	State  string `json:"state"`
	Log    string `json:"log"`
}

// Runner runs the deploy script. Tests replace it.
type Runner func(ctx context.Context, tag, digest string, log io.Writer) error

// ScriptRunner runs script with the tag and digest as its two arguments.
func ScriptRunner(script string) Runner {
	return func(ctx context.Context, tag, digest string, log io.Writer) error {
		cmd := exec.CommandContext(ctx, script, tag, digest)
		cmd.Stdout, cmd.Stderr = log, log
		return cmd.Run()
	}
}

// Server is the HTTP handler.
type Server struct {
	secret  []byte
	run     Runner
	now     func() time.Time
	timeout time.Duration

	mu      sync.Mutex
	current string             // id of the running deploy, "" if none
	deploys map[string]*deploy // by id; only the most recent few are kept
	order   []string
}

type deploy struct {
	mu     sync.Mutex
	status Status
	log    bytes.Buffer
}

// New returns a Server. timeout bounds one deploy.
func New(secret []byte, run Runner, timeout time.Duration) *Server {
	return &Server{secret: secret, run: run, now: time.Now, timeout: timeout, deploys: map[string]*deploy{}}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == http.MethodPost && r.URL.Path == "/deploy":
		s.start(w, r)
	case r.Method == http.MethodGet && len(r.URL.Path) > len("/deploy/") && r.URL.Path[:len("/deploy/")] == "/deploy/":
		s.status(w, r, r.URL.Path[len("/deploy/"):])
	default:
		http.NotFound(w, r)
	}
}

// authentic checks X-Timestamp is recent and X-Signature is the HMAC of
// "<ts>\n<payload>", where payload is the request body (POST) or the path (GET).
func (s *Server) authentic(r *http.Request, payload string) bool {
	ts := r.Header.Get("X-Timestamp")
	sec, err := strconv.ParseInt(ts, 10, 64)
	if err != nil {
		return false
	}
	d := s.now().Sub(time.Unix(sec, 0))
	if d > MaxSkew || d < -MaxSkew {
		return false
	}
	want := Sign(s.secret, ts+"\n"+payload)
	return hmac.Equal([]byte(want), []byte(r.Header.Get("X-Signature")))
}

func (s *Server) start(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, maxBody+1))
	if err != nil || len(body) > maxBody {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if !s.authentic(r, string(body)) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var req Request
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil || !tagRe.MatchString(req.Tag) || !digestRe.MatchString(req.Digest) {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	s.mu.Lock()
	if s.current != "" {
		s.mu.Unlock()
		http.Error(w, "a deploy is already running", http.StatusConflict)
		return
	}
	id := newID()
	d := &deploy{status: Status{ID: id, Tag: req.Tag, Digest: req.Digest, State: StateRunning}}
	s.current = id
	s.deploys[id] = d
	s.order = append(s.order, id)
	for len(s.order) > 10 {
		delete(s.deploys, s.order[0])
		s.order = s.order[1:]
	}
	s.mu.Unlock()

	go s.execute(d, req)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]string{"id": id})
}

func (s *Server) execute(d *deploy, req Request) {
	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()
	err := s.run(ctx, req.Tag, req.Digest, &lockedWriter{d: d})

	d.mu.Lock()
	if err != nil {
		d.status.State = StateFailed
		d.log.WriteString("\ndeploy failed: " + err.Error() + "\n")
	} else {
		d.status.State = StateSucceeded
	}
	d.mu.Unlock()

	s.mu.Lock()
	s.current = ""
	s.mu.Unlock()
}

type lockedWriter struct{ d *deploy }

func (l *lockedWriter) Write(p []byte) (int, error) {
	l.d.mu.Lock()
	defer l.d.mu.Unlock()
	if l.d.log.Len() < maxLogBytes {
		l.d.log.Write(p)
	}
	return len(p), nil
}

func (s *Server) status(w http.ResponseWriter, r *http.Request, id string) {
	if !s.authentic(r, r.URL.Path) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if !idRe.MatchString(id) {
		http.NotFound(w, r)
		return
	}
	s.mu.Lock()
	d := s.deploys[id]
	s.mu.Unlock()
	if d == nil {
		http.NotFound(w, r)
		return
	}
	d.mu.Lock()
	st := d.status
	st.Log = d.log.String()
	d.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(st)
}

func newID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(errors.New("deployhook: no randomness: " + err.Error()))
	}
	return hex.EncodeToString(b)
}
