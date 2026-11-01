package admin

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/jpeg"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mlh5599/muddys-music-cafe/server/internal/storage"
)

const (
	testTeam = "team.example.test"
	testAud  = "aud-tag"
)

type staticKeys map[string]*rsa.PublicKey

func (s staticKeys) Keys(context.Context) (map[string]*rsa.PublicKey, error) { return s, nil }

func mustKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	k, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	return k
}

func b64(v any) string {
	b, _ := json.Marshal(v)
	return base64.RawURLEncoding.EncodeToString(b)
}

func sign(t *testing.T, k *rsa.PrivateKey, hdr, claims map[string]any) string {
	t.Helper()
	signing := b64(hdr) + "." + b64(claims)
	sum := sha256.Sum256([]byte(signing))
	sig, err := rsa.SignPKCS1v15(rand.Reader, k, crypto.SHA256, sum[:])
	if err != nil {
		t.Fatal(err)
	}
	return signing + "." + base64.RawURLEncoding.EncodeToString(sig)
}

func goodClaims() map[string]any {
	return map[string]any{
		"iss": "https://" + testTeam, "aud": []string{testAud},
		"exp": time.Now().Add(time.Hour).Unix(), "email": "manager@example.test",
	}
}

func TestVerifier(t *testing.T) {
	key, other := mustKey(t), mustKey(t)
	v, err := NewVerifier(testTeam, testAud, staticKeys{"k1": &key.PublicKey})
	if err != nil {
		t.Fatal(err)
	}
	hdr := map[string]any{"alg": "RS256", "kid": "k1"}
	override := func(k, val any) map[string]any {
		c := goodClaims()
		c[k.(string)] = val
		return c
	}
	cases := []struct {
		name  string
		token string
		ok    bool
	}{
		{"valid", sign(t, key, hdr, goodClaims()), true},
		{"valid string aud", sign(t, key, hdr, override("aud", testAud)), true},
		{"wrong key", sign(t, other, hdr, goodClaims()), false},
		{"unknown kid", sign(t, key, map[string]any{"alg": "RS256", "kid": "zzz"}, goodClaims()), false},
		{"alg none", b64(map[string]any{"alg": "none", "kid": "k1"}) + "." + b64(goodClaims()) + ".", false},
		{"alg HS256", sign(t, key, map[string]any{"alg": "HS256", "kid": "k1"}, goodClaims()), false},
		{"expired", sign(t, key, hdr, override("exp", time.Now().Add(-time.Minute).Unix())), false},
		{"no exp", sign(t, key, hdr, map[string]any{"iss": "https://" + testTeam, "aud": testAud}), false},
		{"wrong aud", sign(t, key, hdr, override("aud", []string{"other"})), false},
		{"wrong iss", sign(t, key, hdr, override("iss", "https://evil.test")), false},
		{"garbage", "not.a.jwt", false},
		{"empty", "", false},
	}
	for _, c := range cases {
		id, err := v.Verify(context.Background(), c.token)
		if (err == nil) != c.ok {
			t.Errorf("%s: err = %v, want ok=%v", c.name, err, c.ok)
		}
		if c.ok && id.Email != "manager@example.test" {
			t.Errorf("%s: email = %q", c.name, id.Email)
		}
	}
	if _, err := NewVerifier("", "x", nil); err == nil {
		t.Error("empty team domain should be rejected")
	}
}

type env struct {
	h   http.Handler
	key *rsa.PrivateKey
	dir string
}

func newEnv(t *testing.T) *env {
	t.Helper()
	db, err := storage.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	key := mustKey(t)
	v, _ := NewVerifier(testTeam, testAud, staticKeys{"k1": &key.PublicKey})
	dir := filepath.Join(t.TempDir(), "uploads")
	return &env{h: New(Config{DB: db, UploadsDir: dir, Verifier: v}), key: key, dir: dir}
}

func (e *env) token(t *testing.T) string {
	return sign(t, e.key, map[string]any{"alg": "RS256", "kid": "k1"}, goodClaims())
}

// do sends an authenticated, CSRF-compliant request.
func (e *env) do(t *testing.T, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var rd *bytes.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	} else {
		rd = bytes.NewReader(nil)
	}
	r := httptest.NewRequest(method, path, rd)
	r.Header.Set(AccessHeader, e.token(t))
	r.Header.Set(CSRFHeader, CSRFValue)
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	e.h.ServeHTTP(w, r)
	return w
}

func decodeBody[T any](t *testing.T, w *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(w.Body.Bytes(), &v); err != nil {
		t.Fatalf("bad json %q: %v", w.Body.String(), err)
	}
	return v
}

func TestAuthRequired(t *testing.T) {
	e := newEnv(t)
	for _, tc := range []struct{ method, path string }{
		{"GET", "/api/admin/me"}, {"GET", "/api/admin/events"}, {"POST", "/api/admin/events"},
		{"PUT", "/api/admin/settings"}, {"POST", "/api/admin/images"}, {"DELETE", "/api/admin/staff/1"},
		{"GET", "/api/admin/nope"},
	} {
		r := httptest.NewRequest(tc.method, tc.path, nil)
		r.Header.Set(CSRFHeader, CSRFValue)
		w := httptest.NewRecorder()
		e.h.ServeHTTP(w, r)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("%s %s without token: %d", tc.method, tc.path, w.Code)
		}
		r.Header.Set(AccessHeader, "garbage")
		w = httptest.NewRecorder()
		e.h.ServeHTTP(w, r)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("%s %s with bad token: %d", tc.method, tc.path, w.Code)
		}
	}
}

func TestLockedWhenNoVerifier(t *testing.T) {
	db, _ := storage.Open(filepath.Join(t.TempDir(), "t.db"))
	defer db.Close()
	h := New(Config{DB: db, UploadsDir: t.TempDir()})
	r := httptest.NewRequest("GET", "/api/admin/events", nil)
	r.Header.Set(AccessHeader, "anything")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("code = %d", w.Code)
	}
}

func TestDevNoAuth(t *testing.T) {
	db, _ := storage.Open(filepath.Join(t.TempDir(), "t.db"))
	defer db.Close()
	h := New(Config{DB: db, UploadsDir: t.TempDir(), DevNoAuth: true})
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/api/admin/me", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("code = %d", w.Code)
	}
}

func TestCSRF(t *testing.T) {
	e := newEnv(t)
	send := func(mut func(*http.Request)) int {
		r := httptest.NewRequest("POST", "/api/admin/partners", strings.NewReader(`{"name":"X","active":true}`))
		r.Header.Set(AccessHeader, e.token(t))
		mut(r)
		w := httptest.NewRecorder()
		e.h.ServeHTTP(w, r)
		return w.Code
	}
	if c := send(func(r *http.Request) {}); c != http.StatusForbidden {
		t.Errorf("missing header: %d", c)
	}
	if c := send(func(r *http.Request) {
		r.Header.Set(CSRFHeader, CSRFValue)
		r.Header.Set("Origin", "https://evil.test")
	}); c != http.StatusForbidden {
		t.Errorf("cross origin: %d", c)
	}
	if c := send(func(r *http.Request) { r.Header.Set(CSRFHeader, CSRFValue); r.Header.Set("Origin", "http://"+r.Host) }); c != http.StatusCreated {
		t.Errorf("same origin: %d", c)
	}
}

func TestEventsCRUD(t *testing.T) {
	e := newEnv(t)
	w := e.do(t, "POST", "/api/admin/events", map[string]any{
		"title": "  Open Mic  ", "summary": "Come sing.", "badgeText": "Tonight", "badgeStyle": "purple",
		"startsAt": "2026-11-20T02:00:00Z", "endsAt": "2026-11-20T04:00:00-08:00",
		"linkUrl": "https://example.com/x", "linkLabel": "RSVP", "published": true,
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", w.Code, w.Body)
	}
	created := decodeBody[eventBody](t, w)
	if created.Title != "Open Mic" || created.Kind != "event" || *created.EndsAt != "2026-11-20T12:00:00Z" {
		t.Fatalf("unexpected: %+v", created)
	}

	created.Title = "Open Mic Night"
	w = e.do(t, "PUT", "/api/admin/events/1", created)
	if w.Code != http.StatusOK || decodeBody[eventBody](t, w).Title != "Open Mic Night" {
		t.Fatalf("update: %d %s", w.Code, w.Body)
	}
	if w = e.do(t, "GET", "/api/admin/events", nil); len(decodeBody[[]eventBody](t, w)) != 1 {
		t.Fatalf("list: %s", w.Body)
	}
	if w = e.do(t, "DELETE", "/api/admin/events/1", nil); w.Code != http.StatusNoContent {
		t.Fatalf("delete: %d", w.Code)
	}
	if w = e.do(t, "GET", "/api/admin/events/1", nil); w.Code != http.StatusNotFound {
		t.Fatalf("get after delete: %d", w.Code)
	}
	if w = e.do(t, "PUT", "/api/admin/events/999", created); w.Code != http.StatusNotFound {
		t.Fatalf("update missing: %d", w.Code)
	}
}

func TestEventValidation(t *testing.T) {
	e := newEnv(t)
	w := e.do(t, "POST", "/api/admin/events", map[string]any{
		"title": "", "badgeStyle": "green", "startsAt": "tomorrow",
		"linkUrl": "javascript:alert(1)", "link2Url": "https://ok.test", "posterImageId": 42,
		"endsAt": "2026-01-01T00:00:00Z",
	})
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("code = %d", w.Code)
	}
	body := decodeBody[errorBody](t, w)
	for _, f := range []string{"title", "badgeStyle", "startsAt", "linkUrl", "link2Label", "posterImageId"} {
		if body.Field[f] == "" {
			t.Errorf("missing message for %s: %+v", f, body.Field)
		}
	}
	for f, msg := range body.Field {
		if strings.Contains(msg, "null") || strings.Contains(strings.ToLower(msg), "invalid") {
			t.Errorf("%s message is technical: %q", f, msg)
		}
	}
	w = e.do(t, "POST", "/api/admin/events", map[string]any{
		"title": "x", "startsAt": "2026-02-01T00:00:00Z", "endsAt": "2026-01-01T00:00:00Z",
	})
	if decodeBody[errorBody](t, w).Field["endsAt"] == "" {
		t.Error("end before start should be rejected")
	}
}

func TestUnknownFieldsRejected(t *testing.T) {
	e := newEnv(t)
	w := e.do(t, "POST", "/api/admin/partners", map[string]any{"name": "A", "active": true, "id": 5, "admin": true})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("code = %d", w.Code)
	}
}

func TestStaffFlow(t *testing.T) {
	e := newEnv(t)
	var ids []int64
	for _, n := range []string{"DJ Alpha", "DJ Bravo", "DJ Charlie"} {
		w := e.do(t, "POST", "/api/admin/staff", map[string]any{"kind": "dj", "name": n, "active": true})
		if w.Code != http.StatusCreated {
			t.Fatalf("create: %d %s", w.Code, w.Body)
		}
		ids = append(ids, decodeBody[staffBody](t, w).ID)
	}
	if w := e.do(t, "PUT", "/api/admin/staff/order", map[string]any{"kind": "dj", "ids": []int64{ids[2], ids[0], ids[1]}}); w.Code != http.StatusNoContent {
		t.Fatalf("reorder: %d %s", w.Code, w.Body)
	}
	list := decodeBody[[]staffBody](t, e.do(t, "GET", "/api/admin/staff?kind=dj", nil))
	if len(list) != 3 || list[0].ID != ids[2] {
		t.Fatalf("order not applied: %+v", list)
	}
	// hide, and the kind can't be changed by an edit
	b := list[0]
	b.Active, b.Kind = false, "host"
	w := e.do(t, "PUT", "/api/admin/staff/"+itoa(b.ID), b)
	if w.Code != http.StatusOK {
		t.Fatalf("update: %d %s", w.Code, w.Body)
	}
	if got := decodeBody[staffBody](t, w); got.Active || got.Kind != "dj" {
		t.Fatalf("unexpected: %+v", got)
	}
	if w := e.do(t, "GET", "/api/admin/staff", nil); w.Code != http.StatusBadRequest {
		t.Fatalf("missing kind: %d", w.Code)
	}
	if w := e.do(t, "POST", "/api/admin/staff", map[string]any{"kind": "dj", "name": "   "}); w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("blank name: %d", w.Code)
	}
}

func itoa(i int64) string { b, _ := json.Marshal(i); return string(b) }

func TestPartnersFlow(t *testing.T) {
	e := newEnv(t)
	for _, n := range []string{"A", "B"} {
		if w := e.do(t, "POST", "/api/admin/partners", map[string]any{"name": n, "active": true}); w.Code != http.StatusCreated {
			t.Fatalf("create: %d", w.Code)
		}
	}
	if w := e.do(t, "PUT", "/api/admin/partners/order", map[string]any{"ids": []int64{2, 1}}); w.Code != http.StatusNoContent {
		t.Fatalf("reorder: %d %s", w.Code, w.Body)
	}
	list := decodeBody[[]partnerBody](t, e.do(t, "GET", "/api/admin/partners", nil))
	if list[0].Name != "B" {
		t.Fatalf("order: %+v", list)
	}
	if w := e.do(t, "PUT", "/api/admin/partners/1", map[string]any{"name": "A2", "active": false}); w.Code != http.StatusOK {
		t.Fatalf("update: %d", w.Code)
	}
	if w := e.do(t, "DELETE", "/api/admin/partners/2", nil); w.Code != http.StatusNoContent {
		t.Fatalf("delete: %d", w.Code)
	}
}

func TestSettings(t *testing.T) {
	e := newEnv(t)
	w := e.do(t, "PUT", "/api/admin/settings", map[string]string{
		"radio_stream_url": "https://stream.example.test/live", "hiring_text": "Keep your tips", "timezone": "America/Los_Angeles",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("put: %d %s", w.Code, w.Body)
	}
	got := decodeBody[map[string]string](t, e.do(t, "GET", "/api/admin/settings", nil))
	if got["radio_stream_url"] != "https://stream.example.test/live" || len(got) != len(storage.SettingKeys) {
		t.Fatalf("settings: %+v", got)
	}
	for name, body := range map[string]map[string]string{
		"unknown key": {"evil": "x"},
		"bad url":     {"facebook_url": "ftp://x"},
		"bad zone":    {"timezone": "Mars/Base"},
	} {
		if w := e.do(t, "PUT", "/api/admin/settings", body); w.Code == http.StatusOK {
			t.Errorf("%s accepted", name)
		}
	}
}

func upload(t *testing.T, e *env, name string, data []byte) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, _ := mw.CreateFormFile("file", name)
	fw.Write(data)
	mw.Close()
	r := httptest.NewRequest("POST", "/api/admin/images", &buf)
	r.Header.Set(AccessHeader, e.token(t))
	r.Header.Set(CSRFHeader, CSRFValue)
	r.Header.Set("Content-Type", mw.FormDataContentType())
	w := httptest.NewRecorder()
	e.h.ServeHTTP(w, r)
	return w
}

func TestImageUploadAndUse(t *testing.T) {
	e := newEnv(t)
	var jb bytes.Buffer
	jpeg.Encode(&jb, image.NewRGBA(image.Rect(0, 0, 40, 30)), nil)

	w := upload(t, e, "../../evil.php", jb.Bytes())
	if w.Code != http.StatusCreated {
		t.Fatalf("upload: %d %s", w.Code, w.Body)
	}
	img := decodeBody[imageBody](t, w)
	if !strings.HasPrefix(img.URL, "/uploads/") || !strings.HasSuffix(img.URL, ".jpg") || strings.Contains(img.URL, "evil") {
		t.Fatalf("url = %q", img.URL)
	}

	w = e.do(t, "POST", "/api/admin/staff", map[string]any{"kind": "host", "name": "H", "active": true, "photoImageId": img.ID})
	if w.Code != http.StatusCreated || decodeBody[staffBody](t, w).PhotoURL != img.URL {
		t.Fatalf("use image: %d %s", w.Code, w.Body)
	}

	if w := upload(t, e, "x.jpg", []byte("<html>not a picture</html>")); w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("non-image: %d", w.Code)
	}
	if !strings.Contains(upload(t, e, "x.jpg", []byte("nope")).Body.String(), "isn't a picture") {
		t.Error("error should be plain language")
	}
	r := httptest.NewRequest("POST", "/api/admin/images", strings.NewReader("{}"))
	r.Header.Set(AccessHeader, e.token(t))
	r.Header.Set(CSRFHeader, CSRFValue)
	r.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.h.ServeHTTP(rec, r)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("no file: %d", rec.Code)
	}
}

func TestResponsesNotCached(t *testing.T) {
	e := newEnv(t)
	if cc := e.do(t, "GET", "/api/admin/events", nil).Header().Get("Cache-Control"); cc != "no-store" {
		t.Fatalf("Cache-Control = %q", cc)
	}
}
