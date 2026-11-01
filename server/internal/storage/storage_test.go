package storage

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func openTest(t *testing.T) *sql.DB {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestOpenIsIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "x.db")
	for i := 0; i < 2; i++ {
		db, err := Open(path)
		if err != nil {
			t.Fatalf("Open #%d: %v", i, err)
		}
		var v int
		if err := db.QueryRow(`PRAGMA user_version`).Scan(&v); err != nil || v < 1 {
			t.Fatalf("user_version = %d, err %v", v, err)
		}
		db.Close()
	}
}

func TestEvents(t *testing.T) {
	ctx := context.Background()
	s := NewEventStore(openTest(t))
	base := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	past, future, soon := base.Add(-48*time.Hour), base.Add(48*time.Hour), base.Add(24*time.Hour)

	mk := func(title string, starts, ends *time.Time, featured, published bool, kind string) Event {
		e, err := s.Create(ctx, Event{Kind: kind, Title: title, StartsAt: starts, EndsAt: ends, Featured: featured, Published: published})
		if err != nil {
			t.Fatalf("Create %s: %v", title, err)
		}
		return e
	}
	mk("expired", &past, &past, false, true, "")
	mk("hidden", &soon, &future, false, false, "")
	mk("later", &future, &future, false, true, "")
	mk("sooner", &soon, &future, false, true, "")
	mk("undated", nil, nil, false, true, "")
	mk("star", &future, &future, true, true, "")
	mk("roadshow", nil, nil, false, true, EventKindRoad)

	got, err := s.ListPublic(ctx, EventKindEvent, base)
	if err != nil {
		t.Fatal(err)
	}
	var titles []string
	for _, e := range got {
		titles = append(titles, e.Title)
	}
	want := []string{"star", "sooner", "later", "undated"}
	if len(titles) != len(want) {
		t.Fatalf("titles = %v, want %v", titles, want)
	}
	for i := range want {
		if titles[i] != want[i] {
			t.Fatalf("titles = %v, want %v", titles, want)
		}
	}

	road, _ := s.ListPublic(ctx, EventKindRoad, base)
	if len(road) != 1 || road[0].Title != "roadshow" {
		t.Fatalf("road = %+v", road)
	}

	e := got[0]
	e.Title = "renamed"
	e.Published = false
	if err := s.Update(ctx, e); err != nil {
		t.Fatal(err)
	}
	back, _ := s.Get(ctx, e.ID)
	if back.Title != "renamed" || back.Published {
		t.Fatalf("update not applied: %+v", back)
	}
	if err := s.Delete(ctx, e.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get(ctx, e.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get after delete = %v", err)
	}
	if err := s.Update(ctx, Event{ID: 9999, Kind: "event", BadgeStyle: "blue"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Update missing = %v", err)
	}
	if all, _ := s.List(ctx); len(all) != 6 {
		t.Fatalf("List len = %d", len(all))
	}
}

func TestEventCheckConstraints(t *testing.T) {
	s := NewEventStore(openTest(t))
	if _, err := s.Create(context.Background(), Event{Kind: "bogus", Title: "x"}); err == nil {
		t.Fatal("expected invalid kind to be rejected")
	}
	if _, err := s.Create(context.Background(), Event{Title: "x", BadgeStyle: "green"}); err == nil {
		t.Fatal("expected invalid badge style to be rejected")
	}
}

func TestStaff(t *testing.T) {
	ctx := context.Background()
	db := openTest(t)
	s := NewStaffStore(db)

	a, err := s.Create(ctx, StaffMember{Kind: StaffKindDJ, Name: "DJ Andy", Active: true})
	if err != nil {
		t.Fatal(err)
	}
	if a.Slug != "andy" {
		t.Fatalf("slug = %q", a.Slug)
	}
	b, _ := s.Create(ctx, StaffMember{Kind: StaffKindDJ, Name: "Andy!", Active: true})
	if b.Slug != "andy-2" {
		t.Fatalf("dup slug = %q", b.Slug)
	}
	c, _ := s.Create(ctx, StaffMember{Kind: StaffKindDJ, Name: "Cara", Active: false})
	h, _ := s.Create(ctx, StaffMember{Kind: StaffKindHost, Name: "Andy", Active: true})
	if h.Slug != "andy" {
		t.Fatalf("slug is unique per kind, got %q", h.Slug)
	}

	pub, _ := s.List(ctx, StaffKindDJ, false)
	if len(pub) != 2 {
		t.Fatalf("public djs = %d", len(pub))
	}
	adm, _ := s.List(ctx, StaffKindDJ, true)
	if len(adm) != 3 || adm[0].ID != a.ID || adm[2].ID != c.ID {
		t.Fatalf("admin order wrong: %+v", adm)
	}

	if err := s.Reorder(ctx, StaffKindDJ, []int64{c.ID, b.ID, a.ID}); err != nil {
		t.Fatal(err)
	}
	adm, _ = s.List(ctx, StaffKindDJ, true)
	if adm[0].ID != c.ID || adm[2].ID != a.ID {
		t.Fatalf("reorder not applied: %+v", adm)
	}
	if err := s.Reorder(ctx, StaffKindDJ, []int64{h.ID}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("reorder across kinds = %v", err)
	}

	a.Name, a.Bio = "Andy Prime", "one\n\ntwo"
	if err := s.Update(ctx, a); err != nil {
		t.Fatal(err)
	}
	got, _ := s.Get(ctx, a.ID)
	if got.Name != "Andy Prime" || got.Slug != "andy" || got.Bio != "one\n\ntwo" {
		t.Fatalf("update: %+v", got)
	}
	if err := s.Delete(ctx, a.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get(ctx, a.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get after delete = %v", err)
	}
}

func TestPartners(t *testing.T) {
	ctx := context.Background()
	s := NewPartnerStore(openTest(t))
	a, _ := s.Create(ctx, Partner{Name: "A", Active: true})
	b, _ := s.Create(ctx, Partner{Name: "B", Active: true})
	c, _ := s.Create(ctx, Partner{Name: "C", Active: false})
	if a.SortOrder >= b.SortOrder || b.SortOrder >= c.SortOrder {
		t.Fatalf("orders %d %d %d", a.SortOrder, b.SortOrder, c.SortOrder)
	}
	if l, _ := s.List(ctx, false); len(l) != 2 {
		t.Fatalf("public = %d", len(l))
	}
	if err := s.Reorder(ctx, []int64{c.ID, b.ID, a.ID}); err != nil {
		t.Fatal(err)
	}
	l, _ := s.List(ctx, true)
	if l[0].ID != c.ID {
		t.Fatalf("order: %+v", l)
	}
	b.Name = "B2"
	if err := s.Update(ctx, b); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete(ctx, a.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete(ctx, a.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("second delete = %v", err)
	}
}

func TestSettings(t *testing.T) {
	ctx := context.Background()
	s := NewSettingsStore(openTest(t))
	if v, err := s.Get(ctx, SettingFlickrURL); err != nil || v != "" {
		t.Fatalf("unset = %q, %v", v, err)
	}
	if err := s.SetMany(ctx, map[string]string{SettingFlickrURL: "https://f", SettingTimezone: "America/Los_Angeles"}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetMany(ctx, map[string]string{SettingFlickrURL: "https://g"}); err != nil {
		t.Fatal(err)
	}
	all, _ := s.All(ctx)
	if all[SettingFlickrURL] != "https://g" || len(all) != 2 {
		t.Fatalf("all = %v", all)
	}
	err := s.SetMany(ctx, map[string]string{SettingYouTubeURL: "x", "evil": "y"})
	if !errors.Is(err, ErrUnknownSetting) {
		t.Fatalf("unknown key = %v", err)
	}
	if v, _ := s.Get(ctx, SettingYouTubeURL); v != "" {
		t.Fatal("partial write leaked")
	}
	if _, err := s.Get(ctx, "evil"); !errors.Is(err, ErrUnknownSetting) {
		t.Fatalf("Get unknown = %v", err)
	}
}

func TestImagesAndReferences(t *testing.T) {
	ctx := context.Background()
	db := openTest(t)
	imgs, events := NewImageStore(db), NewEventStore(db)
	img, err := imgs.Create(ctx, Image{Filename: "a.jpg", ContentType: "image/jpeg", SizeBytes: 10, Width: 2, Height: 3})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := imgs.Create(ctx, Image{Filename: "a.jpg", ContentType: "image/jpeg"}); err == nil {
		t.Fatal("duplicate filename accepted")
	}
	got, err := imgs.Get(ctx, img.ID)
	if err != nil || got.Width != 2 {
		t.Fatalf("Get: %+v %v", got, err)
	}
	e, _ := events.Create(ctx, Event{Title: "t", PosterImageID: &img.ID, Published: true})
	if e.PosterImageID == nil {
		t.Fatal("poster not stored")
	}
	if err := imgs.Delete(ctx, img.ID); err != nil {
		t.Fatal(err)
	}
	e, _ = events.Get(ctx, e.ID)
	if e.PosterImageID != nil {
		t.Fatal("poster reference not cleared on image delete")
	}
	bad := int64(424242)
	if _, err := events.Create(ctx, Event{Title: "t", PosterImageID: &bad}); err == nil {
		t.Fatal("foreign key not enforced")
	}
	if _, err := imgs.Get(ctx, img.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get deleted = %v", err)
	}
}
