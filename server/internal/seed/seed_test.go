package seed

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mlh5599/muddys-music-cafe/server/internal/storage"
)

func TestRun(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	db, err := storage.Open(filepath.Join(dir, "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	uploads := filepath.Join(dir, "uploads")

	if err := Run(ctx, db, uploads); err != nil {
		t.Fatalf("Run: %v", err)
	}

	staff := storage.NewStaffStore(db)
	djs, _ := staff.List(ctx, storage.StaffKindDJ, false)
	hosts, _ := staff.List(ctx, storage.StaffKindHost, false)
	if len(djs) != 22 || len(hosts) != 28 {
		t.Fatalf("djs=%d hosts=%d", len(djs), len(hosts))
	}
	for _, m := range append(djs, hosts...) {
		if m.PhotoImageID == nil || m.Slug == "" {
			t.Errorf("%s: photo/slug missing", m.Name)
		}
	}
	if djs[0].Slug != "andy" {
		t.Errorf("first dj slug = %q", djs[0].Slug)
	}
	partners, _ := storage.NewPartnerStore(db).List(ctx, false)
	if len(partners) != 84 || partners[0].Name != "Gacha Shack" {
		t.Errorf("partners = %d", len(partners))
	}

	events := storage.NewEventStore(db)
	pub, _ := events.ListPublic(ctx, storage.EventKindEvent, time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC))
	if len(pub) != 4 || pub[0].Title != "Halloween Block Party" {
		t.Errorf("public events = %+v", pub)
	}
	if road, _ := events.ListPublic(ctx, storage.EventKindRoad, time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)); len(road) != 1 {
		t.Errorf("road = %d", len(road))
	}

	settings, _ := storage.NewSettingsStore(db).All(ctx)
	if settings[storage.SettingRadioStreamURL] == "" || settings[storage.SettingClubSLurl] != "" {
		t.Errorf("settings = %v", settings)
	}

	files, _ := os.ReadDir(uploads)
	if len(files) != 50+5 {
		t.Errorf("upload files = %d", len(files))
	}

	if err := Run(ctx, db, uploads); !errors.Is(err, ErrNotEmpty) {
		t.Errorf("second Run = %v, want ErrNotEmpty", err)
	}
}
