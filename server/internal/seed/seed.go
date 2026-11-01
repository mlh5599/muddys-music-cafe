// Package seed imports the original proof-of-concept content (DJs, hosts,
// partners, events and settings) into an empty database.
package seed

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/mlh5599/muddys-music-cafe/server/internal/storage"
)

//go:embed data
var data embed.FS

// ErrNotEmpty means the database already has content, so nothing was imported.
var ErrNotEmpty = errors.New("seed: database already has content; refusing to import")

type staffEntry struct {
	Kind     string   `json:"kind"`
	Name     string   `json:"name"`
	SLName   string   `json:"sl_name"`
	Photo    string   `json:"photo"`
	Bio      []string `json:"bio"`
	Schedule []string `json:"schedule"`
}

// Run imports the seed content into db and writes image files to uploadsDir.
// It does nothing and returns ErrNotEmpty if any staff, partner or event row
// already exists, so it can never overwrite real content. It is not
// transactional: if it fails part way, delete the database file and run again.
func Run(ctx context.Context, db *sql.DB, uploadsDir string) error {
	var n int
	if err := db.QueryRowContext(ctx,
		`SELECT (SELECT COUNT(*) FROM staff_members) + (SELECT COUNT(*) FROM partners) + (SELECT COUNT(*) FROM events)`).Scan(&n); err != nil {
		return fmt.Errorf("seed: check empty: %w", err)
	}
	if n > 0 {
		return ErrNotEmpty
	}
	if err := os.MkdirAll(uploadsDir, 0o750); err != nil {
		return fmt.Errorf("seed: create uploads dir: %w", err)
	}

	imgs := storage.NewImageStore(db)
	cache := map[string]int64{}
	importImage := func(rel string) (*int64, error) {
		if rel == "" {
			return nil, nil
		}
		if id, ok := cache[rel]; ok {
			return &id, nil
		}
		b, err := data.ReadFile(path.Join("data/images", rel))
		if err != nil {
			return nil, fmt.Errorf("seed: read image %s: %w", rel, err)
		}
		cfg, format, err := image.DecodeConfig(strings.NewReader(string(b)))
		if err != nil {
			return nil, fmt.Errorf("seed: decode image %s: %w", rel, err)
		}
		ext := map[string]string{"jpeg": ".jpg", "png": ".png"}[format]
		sum := sha256.Sum256(b)
		name := hex.EncodeToString(sum[:])[:24] + ext
		if err := os.WriteFile(filepath.Join(uploadsDir, name), b, 0o640); err != nil {
			return nil, fmt.Errorf("seed: write image %s: %w", name, err)
		}
		img, err := imgs.Create(ctx, storage.Image{
			Filename: name, ContentType: "image/" + format, SizeBytes: int64(len(b)), Width: cfg.Width, Height: cfg.Height,
		})
		if err != nil {
			return nil, err
		}
		cache[rel] = img.ID
		return &img.ID, nil
	}

	// Staff.
	var staff []staffEntry
	if err := readJSON("data/staff.json", &staff); err != nil {
		return err
	}
	ss := storage.NewStaffStore(db)
	for _, e := range staff {
		photo, err := importImage(e.Photo)
		if err != nil {
			return err
		}
		if _, err := ss.Create(ctx, storage.StaffMember{
			Kind: e.Kind, Name: e.Name, SLName: e.SLName,
			Bio: strings.Join(e.Bio, "\n\n"), Schedule: strings.Join(e.Schedule, "\n"),
			PhotoImageID: photo, Active: true,
		}); err != nil {
			return err
		}
	}

	// Partners.
	var partners []string
	if err := readJSON("data/partners.json", &partners); err != nil {
		return err
	}
	ps := storage.NewPartnerStore(db)
	for _, name := range partners {
		if _, err := ps.Create(ctx, storage.Partner{Name: name, Active: true}); err != nil {
			return err
		}
	}

	// Events. Times are Second Life time (US Pacific), stored as UTC.
	slt, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		return fmt.Errorf("seed: load timezone: %w", err)
	}
	at := func(m time.Month, d, h, min int) *time.Time {
		t := time.Date(2026, m, d, h, min, 0, 0, slt).UTC()
		return &t
	}
	events := []struct {
		poster string
		e      storage.Event
	}{
		{"award4.jpg", storage.Event{
			Title: "Halloween Block Party", BadgeText: "Oct 29 – 31", BadgeStyle: "orange", Featured: true,
			StartsAt: at(time.October, 29, 0, 0), EndsAt: at(time.October, 31, 23, 59),
			Summary: "Three nights of trick-or-treat, costumes and spooky sets from our DJs. Come dressed up.",
		}},
		{"award3.jpg", storage.Event{
			Title: "Monster Mash-Up Photo Contest", BadgeText: "Oct 29 – Nov 1", BadgeStyle: "purple",
			StartsAt: at(time.October, 29, 0, 0), EndsAt: at(time.November, 1, 23, 59),
			Summary: "Dress to distress, pose to impress. Prizes of L$5,000, L$2,500 and L$1,000. Submit to our Flickr group. Original SL photography only, no AI.",
			LinkURL: "https://www.flickr.com/groups/muddys/", LinkLabel: "Submit on Flickr",
		}},
		{"award1.jpg", storage.Event{
			Title: "Road to Full Circle", BadgeText: "Roadshow", BadgeStyle: "blue",
			Summary: "We're the official sponsor of Premier Wrestling's Full Circle. Everyone at the roadshow gets a free Premier Wrestling T-shirt and a chance to win a free ticket.",
			LinkURL: "https://maps.secondlife.com/secondlife/Premier%20Virtual%20Entertainment/157/153/23", LinkLabel: "Teleport to the roadshow",
		}},
		{"award2.jpg", storage.Event{
			Title: "The 4th Wall Podcast", BadgeText: "In the media", BadgeStyle: "pink",
			Summary: "BK and Paul sit down with owner Bridget Hammill to talk about Muddy's, the music, and 16 years on the grid.",
			LinkURL: "https://www.youtube.com/watch?v=D2rIlQpxip8", LinkLabel: "Watch on YouTube",
		}},
		{"cloud-nine.jpg", storage.Event{
			Kind: storage.EventKindRoad, Title: "Cloud Nine", BadgeStyle: "blue",
			StartsAt: at(time.October, 25, 13, 45), EndsAt: at(time.October, 25, 16, 0),
			Summary: "Muddy's hits the road with guest sets at venues across the grid. Next stop: Cloud Nine, with DJ John and host Amfie streaming live on Sunday, October 25th, 1:45 – 4:00 PM SLT.",
			LinkURL: "https://maps.secondlife.com/secondlife/GoodVibes/125/127/21", LinkLabel: "Teleport to Cloud Nine",
			Link2URL: "https://soundcloud.com/user-680512645/muddys-radio-cloud-9", Link2Label: "Hear it on SoundCloud",
		}},
	}
	es := storage.NewEventStore(db)
	for _, ev := range events {
		poster, err := importImage(ev.poster)
		if err != nil {
			return err
		}
		ev.e.PosterImageID = poster
		ev.e.Published = true
		if _, err := es.Create(ctx, ev.e); err != nil {
			return err
		}
	}

	// Settings.
	return storage.NewSettingsStore(db).SetMany(ctx, map[string]string{
		storage.SettingRadioStreamURL: "http://muddys.digistream.info:20398/stream",
		storage.SettingRadioStatusURL: "http://muddys.digistream.info:20398/index.html?sid=1",
		storage.SettingSLGroupURL:     "http://world.secondlife.com/group/6ffdf9ce-7581-3731-e601-81ee5f9b6842",
		storage.SettingFacebookURL:    "https://www.facebook.com/Muddysmusiccafe",
		storage.SettingFlickrURL:      "https://www.flickr.com/groups/muddys/",
		storage.SettingYouTubeURL:     "https://www.youtube.com/watch?v=D2rIlQpxip8",
		storage.SettingHiringText:     "Must have experience\nAble to work a busy 24/7 club\nPG venue",
		storage.SettingTimezone:       "America/Los_Angeles",
	})
}

func readJSON(name string, v any) error {
	b, err := data.ReadFile(name)
	if err != nil {
		return fmt.Errorf("seed: read %s: %w", name, err)
	}
	if err := json.Unmarshal(b, v); err != nil {
		return fmt.Errorf("seed: parse %s: %w", name, err)
	}
	return nil
}
