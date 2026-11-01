package site

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mlh5599/muddys-music-cafe/server/internal/seed"
	"github.com/mlh5599/muddys-music-cafe/server/internal/storage"
)

func newSeededSite(t *testing.T, now time.Time) (*Site, string) {
	t.Helper()
	dir := t.TempDir()
	db, err := storage.Open(filepath.Join(dir, "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	uploads := filepath.Join(dir, "uploads")
	if err := seed.Run(context.Background(), db, uploads); err != nil {
		t.Fatal(err)
	}
	s, err := New(Config{DB: db, UploadsDir: uploads, Now: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	return s, uploads
}

func get(s *Site, path string, hdr ...string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	for i := 0; i+1 < len(hdr); i += 2 {
		req.Header.Set(hdr[i], hdr[i+1])
	}
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	return rec
}

func TestHome(t *testing.T) {
	s, _ := newSeededSite(t, time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC))
	rec := get(s, "/")
	if rec.Code != 200 {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	body := rec.Body.String()
	for _, want := range []string{
		"Halloween Block Party", "Oct 29 – 31", "Monster Mash-Up Photo Contest", "Submit on Flickr →",
		"Gacha Shack", "Our 22 DJs", "Our 28 Hosts", `data-stream="/radio/stream"`,
		"Must have experience", "/uploads/",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("home missing %q", want)
		}
	}
	// The Oct 25 road stop is over, so its section must be gone.
	if strings.Contains(body, "Muddy's on the road") {
		t.Error("expired road block still shown")
	}
	if strings.Contains(body, "Teleport to Muddy") {
		t.Error("club teleport button shown with no SLurl set")
	}
	if rec.Header().Get("ETag") == "" || rec.Header().Get("Cache-Control") != "no-cache" {
		t.Errorf("caching headers: %v", rec.Header())
	}
	if again := get(s, "/", "If-None-Match", rec.Header().Get("ETag")); again.Code != http.StatusNotModified {
		t.Errorf("conditional GET = %d", again.Code)
	}
}

func TestHomeShowsRoadBlockBeforeItEnds(t *testing.T) {
	s, _ := newSeededSite(t, time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC))
	body := get(s, "/").Body.String()
	if !strings.Contains(body, "Muddy's on the road") || !strings.Contains(body, "Teleport to Cloud Nine") {
		t.Error("road block missing")
	}
}

func TestHomeEmptyDatabase(t *testing.T) {
	db, err := storage.Open(filepath.Join(t.TempDir(), "e.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	s, err := New(Config{DB: db, UploadsDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	rec := get(s, "/")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "No events are scheduled") {
		t.Fatalf("empty home: %d", rec.Code)
	}
}

func TestStaffPages(t *testing.T) {
	s, _ := newSeededSite(t, time.Now())
	for path, want := range map[string][]string{
		"/djs":   {`id="dj-andy"`, "DJ Andy", "<strong>22</strong>", "Find a DJ"},
		"/hosts": {`id="host-`, "<strong>28</strong>", "Second Life name", "Catch ", "Find a Host"},
	} {
		rec := get(s, path)
		if rec.Code != 200 {
			t.Fatalf("%s: %d", path, rec.Code)
		}
		for _, w := range want {
			if !strings.Contains(rec.Body.String(), w) {
				t.Errorf("%s missing %q", path, w)
			}
		}
	}
}

func TestEscapesContent(t *testing.T) {
	s, _ := newSeededSite(t, time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC))
	st := storage.NewStaffStore(s.cfg.DB)
	if _, err := st.Create(context.Background(), storage.StaffMember{
		Kind: storage.StaffKindDJ, Name: `<script>alert(1)</script>`, Bio: `<img src=x onerror=alert(1)>`, Active: true,
	}); err != nil {
		t.Fatal(err)
	}
	body := get(s, "/djs").Body.String()
	if strings.Contains(body, "<script>alert") || strings.Contains(body, "<img src=x") {
		t.Error("unescaped HTML in page")
	}
}

func TestHiddenStaffNotShown(t *testing.T) {
	s, _ := newSeededSite(t, time.Now())
	st := storage.NewStaffStore(s.cfg.DB)
	djs, _ := st.List(context.Background(), storage.StaffKindDJ, true)
	djs[0].Active = false
	if err := st.Update(context.Background(), djs[0]); err != nil {
		t.Fatal(err)
	}
	body := get(s, "/djs").Body.String()
	if strings.Contains(body, `id="dj-andy"`) || !strings.Contains(body, "<strong>21</strong>") {
		t.Error("hidden DJ still shown")
	}
}

func TestStaticAndUploads(t *testing.T) {
	s, uploads := newSeededSite(t, time.Now())

	rec := get(s, "/static/styles.css")
	if rec.Code != 200 || !strings.Contains(rec.Header().Get("Cache-Control"), "immutable") {
		t.Errorf("static: %d %v", rec.Code, rec.Header())
	}
	files, _ := os.ReadDir(uploads)
	rec = get(s, "/uploads/"+files[0].Name())
	if rec.Code != 200 || !strings.HasPrefix(rec.Header().Get("Content-Type"), "image/") {
		t.Errorf("upload: %d %q", rec.Code, rec.Header().Get("Content-Type"))
	}
	for _, p := range []string{"/uploads/missing.jpg", "/uploads/..%2Fsecret.jpg", "/uploads/.hidden.jpg", "/uploads/x.html", "/uploads/"} {
		if c := get(s, p).Code; c != 404 {
			t.Errorf("%s = %d, want 404", p, c)
		}
	}
	if c := get(s, "/nope").Code; c != 404 {
		t.Errorf("/nope = %d", c)
	}
	if c := get(s, "/index.html").Code; c != 404 {
		t.Errorf("/index.html = %d", c)
	}
}

func TestDateRange(t *testing.T) {
	loc, _ := time.LoadLocation("America/Los_Angeles")
	d := func(m time.Month, day int) *time.Time { t := time.Date(2026, m, day, 12, 0, 0, 0, loc); return &t }
	for _, c := range []struct {
		a, b *time.Time
		want string
	}{
		{nil, nil, ""}, {d(10, 29), nil, "Oct 29"}, {d(10, 29), d(10, 29), "Oct 29"},
		{d(10, 29), d(10, 31), "Oct 29 – 31"}, {d(10, 29), d(11, 1), "Oct 29 – Nov 1"},
	} {
		if got := dateRange(c.a, c.b, loc); got != c.want {
			t.Errorf("dateRange = %q, want %q", got, c.want)
		}
	}
}

func TestTeaser(t *testing.T) {
	if teaser(nil) != "Bio coming soon." {
		t.Error("empty teaser")
	}
	long := strings.Repeat("word ", 40)
	got := teaser([]string{long})
	if !strings.HasSuffix(got, "…") || len([]rune(got)) > 121 {
		t.Errorf("teaser = %q", got)
	}
}

func TestRadioRelay(t *testing.T) {
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Icy-MetaData") != "" {
			t.Error("relay asked upstream for metadata")
		}
		w.Header().Set("Content-Type", "audio/mpeg")
		w.Header().Set("icy-name", "secret")
		w.Write([]byte("ID3-fake-audio-bytes"))
	}))
	defer up.Close()

	s, _ := newSeededSite(t, time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC))
	if err := s.settings.SetMany(context.Background(), map[string]string{storage.SettingRadioStreamURL: up.URL + "/stream"}); err != nil {
		t.Fatal(err)
	}
	rec := get(s, "/radio/stream?t=123")
	if rec.Code != 200 || rec.Body.String() != "ID3-fake-audio-bytes" {
		t.Fatalf("relay = %d %q", rec.Code, rec.Body)
	}
	if rec.Header().Get("Content-Type") != "audio/mpeg" || rec.Header().Get("Cache-Control") != "no-store" {
		t.Errorf("headers: %v", rec.Header())
	}
	if rec.Header().Get("icy-name") != "" {
		t.Error("upstream headers leaked")
	}
}

func TestRadioRelayUnavailable(t *testing.T) {
	s, _ := newSeededSite(t, time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC))
	ctx := context.Background()

	if err := s.settings.SetMany(ctx, map[string]string{storage.SettingRadioStreamURL: ""}); err != nil {
		t.Fatal(err)
	}
	if rec := get(s, "/radio/stream"); rec.Code != http.StatusServiceUnavailable {
		t.Errorf("unset stream = %d", rec.Code)
	}
	if body := get(s, "/").Body.String(); strings.Contains(body, "data-stream") {
		t.Error("player given a stream address while none is set")
	}

	down := httptest.NewServer(http.NotFoundHandler())
	defer down.Close()
	if err := s.settings.SetMany(ctx, map[string]string{storage.SettingRadioStreamURL: down.URL}); err != nil {
		t.Fatal(err)
	}
	if rec := get(s, "/radio/stream"); rec.Code != http.StatusBadGateway {
		t.Errorf("upstream 404 = %d", rec.Code)
	}
}

func TestRadioRelayListenerCap(t *testing.T) {
	s, _ := newSeededSite(t, time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC))
	for i := 0; i < maxListeners; i++ {
		s.listeners <- struct{}{}
	}
	if rec := get(s, "/radio/stream"); rec.Code != http.StatusServiceUnavailable {
		t.Errorf("over cap = %d", rec.Code)
	}
}
