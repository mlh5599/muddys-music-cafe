package deployhook

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

var secret = []byte("test-secret")

const (
	goodTag    = "v2026.11.01.1"
	goodDigest = "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
)

func signed(t *testing.T, method, path, body string, at time.Time, key []byte) *http.Request {
	t.Helper()
	ts := strconv.FormatInt(at.Unix(), 10)
	payload := body
	if method == http.MethodGet {
		payload = path
	}
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("X-Timestamp", ts)
	req.Header.Set("X-Signature", Sign(key, ts+"\n"+payload))
	return req
}

func body(tag, digest string) string {
	b, _ := json.Marshal(Request{Tag: tag, Digest: digest})
	return string(b)
}

func do(s *Server, r *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, r)
	return rec
}

func waitState(t *testing.T, s *Server, id string, want string) Status {
	t.Helper()
	for i := 0; i < 200; i++ {
		rec := do(s, signed(t, "GET", "/deploy/"+id, "", time.Now(), secret))
		var st Status
		json.Unmarshal(rec.Body.Bytes(), &st)
		if st.State == want {
			return st
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("deploy never reached %q", want)
	return Status{}
}

func TestDeploySucceeds(t *testing.T) {
	var gotTag, gotDigest string
	s := New(secret, func(_ context.Context, tag, digest string, log io.Writer) error {
		gotTag, gotDigest = tag, digest
		io.WriteString(log, "pulled\n")
		return nil
	}, time.Minute)
	rec := do(s, signed(t, "POST", "/deploy", body(goodTag, goodDigest), time.Now(), secret))
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	var out map[string]string
	json.Unmarshal(rec.Body.Bytes(), &out)
	st := waitState(t, s, out["id"], StateSucceeded)
	if gotTag != goodTag || gotDigest != goodDigest || !strings.Contains(st.Log, "pulled") {
		t.Errorf("ran with %q %q, status %+v", gotTag, gotDigest, st)
	}
}

func TestDeployFailureIsReported(t *testing.T) {
	s := New(secret, func(context.Context, string, string, io.Writer) error { return errors.New("unhealthy") }, time.Minute)
	rec := do(s, signed(t, "POST", "/deploy", body(goodTag, goodDigest), time.Now(), secret))
	var out map[string]string
	json.Unmarshal(rec.Body.Bytes(), &out)
	st := waitState(t, s, out["id"], StateFailed)
	if !strings.Contains(st.Log, "unhealthy") {
		t.Errorf("log %q", st.Log)
	}
}

func TestRejectsUnauthenticated(t *testing.T) {
	ran := false
	s := New(secret, func(context.Context, string, string, io.Writer) error { ran = true; return nil }, time.Minute)
	b := body(goodTag, goodDigest)
	for name, req := range map[string]*http.Request{
		"wrong key":    signed(t, "POST", "/deploy", b, time.Now(), []byte("other")),
		"stale":        signed(t, "POST", "/deploy", b, time.Now().Add(-time.Hour), secret),
		"future":       signed(t, "POST", "/deploy", b, time.Now().Add(time.Hour), secret),
		"no signature": httptest.NewRequest("POST", "/deploy", strings.NewReader(b)),
		"tampered body": func() *http.Request {
			r := signed(t, "POST", "/deploy", b, time.Now(), secret)
			r.Body = io.NopCloser(strings.NewReader(body("v2026.11.01.2", goodDigest)))
			return r
		}(),
	} {
		if rec := do(s, req); rec.Code != http.StatusUnauthorized {
			t.Errorf("%s: status %d, want 401", name, rec.Code)
		}
	}
	time.Sleep(20 * time.Millisecond)
	if ran {
		t.Error("deploy ran for an unauthenticated request")
	}
}

func TestRejectsBadInput(t *testing.T) {
	s := New(secret, func(context.Context, string, string, io.Writer) error { t.Error("ran"); return nil }, time.Minute)
	for name, b := range map[string]string{
		"bad tag":       body("latest", goodDigest),
		"shell in tag":  body("v2026.11.01.1;reboot", goodDigest),
		"short digest":  body(goodTag, "sha256:abc"),
		"tag as digest": body(goodTag, goodTag),
		"extra field":   `{"tag":"` + goodTag + `","digest":"` + goodDigest + `","image":"evil/image"}`,
		"not json":      `tag=v1`,
		"empty":         ``,
		"huge":          strings.Repeat("x", maxBody+10),
	} {
		if rec := do(s, signed(t, "POST", "/deploy", b, time.Now(), secret)); rec.Code != http.StatusBadRequest {
			t.Errorf("%s: status %d, want 400", name, rec.Code)
		}
	}
}

func TestOneDeployAtATime(t *testing.T) {
	release := make(chan struct{})
	s := New(secret, func(context.Context, string, string, io.Writer) error { <-release; return nil }, time.Minute)
	first := do(s, signed(t, "POST", "/deploy", body(goodTag, goodDigest), time.Now(), secret))
	if first.Code != http.StatusAccepted {
		t.Fatalf("first: %d", first.Code)
	}
	if second := do(s, signed(t, "POST", "/deploy", body("v2026.11.01.2", goodDigest), time.Now(), secret)); second.Code != http.StatusConflict {
		t.Errorf("second concurrent deploy = %d, want 409", second.Code)
	}
	close(release)
	var out map[string]string
	json.Unmarshal(first.Body.Bytes(), &out)
	waitState(t, s, out["id"], StateSucceeded)
	// Once finished, a new deploy is accepted again.
	if third := do(s, signed(t, "POST", "/deploy", body("v2026.11.01.2", goodDigest), time.Now(), secret)); third.Code != http.StatusAccepted {
		t.Errorf("deploy after completion = %d", third.Code)
	}
}

func TestStatusNeedsAuthAndKnownID(t *testing.T) {
	s := New(secret, func(context.Context, string, string, io.Writer) error { return nil }, time.Minute)
	if rec := do(s, httptest.NewRequest("GET", "/deploy/"+strings.Repeat("a", 32), nil)); rec.Code != http.StatusUnauthorized {
		t.Errorf("unauthenticated status = %d", rec.Code)
	}
	if rec := do(s, signed(t, "GET", "/deploy/"+strings.Repeat("a", 32), "", time.Now(), secret)); rec.Code != http.StatusNotFound {
		t.Errorf("unknown id = %d", rec.Code)
	}
	if rec := do(s, signed(t, "GET", "/deploy/../etc", "", time.Now(), secret)); rec.Code == http.StatusOK {
		t.Error("path traversal id accepted")
	}
}

func TestOtherRoutes(t *testing.T) {
	s := New(secret, nil, time.Minute)
	for _, p := range []string{"/", "/metrics", "/deploy/"} {
		if rec := do(s, httptest.NewRequest("GET", p, nil)); rec.Code != http.StatusNotFound {
			t.Errorf("GET %s = %d, want 404", p, rec.Code)
		}
	}
}
