package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// Known setting keys. Only these can be stored, so a typo or a hostile request
// can't create arbitrary rows. Values are plain text.
const (
	SettingRadioStreamURL = "radio_stream_url" // audio stream played by the site's player
	SettingRadioStatusURL = "radio_status_url" // human-facing stream page ("open in a new tab")
	SettingClubSLurl      = "club_slurl"       // teleport link to the club itself
	SettingSLGroupURL     = "sl_group_url"
	SettingFacebookURL    = "facebook_url"
	SettingFlickrURL      = "flickr_url"
	SettingYouTubeURL     = "youtube_url"
	SettingHiringText     = "hiring_text" // "you keep all the tips" checklist, one item per line
	SettingTimezone       = "timezone"    // IANA zone used to display event times (Second Life Time)
)

// SettingKeys lists every valid setting key.
var SettingKeys = []string{
	SettingRadioStreamURL, SettingRadioStatusURL, SettingClubSLurl, SettingSLGroupURL,
	SettingFacebookURL, SettingFlickrURL, SettingYouTubeURL, SettingHiringText, SettingTimezone,
}

// ErrUnknownSetting is returned when a key is not in SettingKeys.
var ErrUnknownSetting = errors.New("storage: unknown setting key")

// SettingsStore persists the settings table.
type SettingsStore struct{ db *sql.DB }

func NewSettingsStore(db *sql.DB) *SettingsStore { return &SettingsStore{db: db} }

func validKey(key string) bool {
	for _, k := range SettingKeys {
		if k == key {
			return true
		}
	}
	return false
}

// All returns every stored setting. Keys never saved are absent from the map.
func (s *SettingsStore) All(ctx context.Context) (map[string]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT key, value FROM settings`)
	if err != nil {
		return nil, fmt.Errorf("storage: list settings: %w", err)
	}
	defer rows.Close()

	out := make(map[string]string)
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, fmt.Errorf("storage: scan setting: %w", err)
		}
		out[k] = v
	}
	return out, rows.Err()
}

// Get returns one setting, or "" if it has never been set.
func (s *SettingsStore) Get(ctx context.Context, key string) (string, error) {
	if !validKey(key) {
		return "", ErrUnknownSetting
	}
	var v string
	err := s.db.QueryRowContext(ctx, `SELECT value FROM settings WHERE key = ?`, key).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("storage: get setting: %w", err)
	}
	return v, nil
}

// SetMany saves the given settings atomically. Any unknown key aborts the
// whole write.
func (s *SettingsStore) SetMany(ctx context.Context, values map[string]string) error {
	for k := range values {
		if !validKey(k) {
			return fmt.Errorf("%w: %q", ErrUnknownSetting, k)
		}
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("storage: begin settings write: %w", err)
	}
	defer tx.Rollback()

	ts := now()
	for k, v := range values {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO settings (key, value, updated_at) VALUES (?, ?, ?)
			 ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`,
			k, v, ts); err != nil {
			return fmt.Errorf("storage: set setting %s: %w", k, err)
		}
	}
	return tx.Commit()
}
