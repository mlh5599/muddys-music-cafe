package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Event kinds.
const (
	EventKindEvent = "event"
	EventKindRoad  = "road"
)

// Event is one row of events.
type Event struct {
	ID            int64
	Kind          string
	Title         string
	Summary       string
	BadgeText     string
	BadgeStyle    string
	StartsAt      *time.Time
	EndsAt        *time.Time
	PosterImageID *int64
	LinkURL       string
	LinkLabel     string
	Link2URL      string
	Link2Label    string
	Featured      bool
	Published     bool
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// EventStore persists events.
type EventStore struct{ db *sql.DB }

func NewEventStore(db *sql.DB) *EventStore { return &EventStore{db: db} }

const eventColumns = `id, kind, title, summary, badge_text, badge_style, starts_at, ends_at, poster_image_id,
	link_url, link_label, link2_url, link2_label, featured, published, created_at, updated_at`

func (s *EventStore) Create(ctx context.Context, e Event) (Event, error) {
	if e.Kind == "" {
		e.Kind = EventKindEvent
	}
	if e.BadgeStyle == "" {
		e.BadgeStyle = "blue"
	}
	ts := now()
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO events (kind, title, summary, badge_text, badge_style, starts_at, ends_at, poster_image_id,
			link_url, link_label, link2_url, link2_label, featured, published, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		e.Kind, e.Title, e.Summary, e.BadgeText, e.BadgeStyle, nullTime(e.StartsAt), nullTime(e.EndsAt), nullInt(e.PosterImageID),
		e.LinkURL, e.LinkLabel, e.Link2URL, e.Link2Label, boolToInt(e.Featured), boolToInt(e.Published), ts, ts)
	if err != nil {
		return Event{}, fmt.Errorf("storage: insert event: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Event{}, fmt.Errorf("storage: get event id: %w", err)
	}
	return s.Get(ctx, id)
}

func (s *EventStore) Get(ctx context.Context, id int64) (Event, error) {
	row := s.db.QueryRowContext(ctx, `SELECT `+eventColumns+` FROM events WHERE id = ?`, id)
	e, err := scanEvent(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Event{}, ErrNotFound
	}
	return e, err
}

func (s *EventStore) Update(ctx context.Context, e Event) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE events SET kind = ?, title = ?, summary = ?, badge_text = ?, badge_style = ?, starts_at = ?, ends_at = ?,
			poster_image_id = ?, link_url = ?, link_label = ?, link2_url = ?, link2_label = ?, featured = ?, published = ?,
			updated_at = ?
		 WHERE id = ?`,
		e.Kind, e.Title, e.Summary, e.BadgeText, e.BadgeStyle, nullTime(e.StartsAt), nullTime(e.EndsAt),
		nullInt(e.PosterImageID), e.LinkURL, e.LinkLabel, e.Link2URL, e.Link2Label, boolToInt(e.Featured), boolToInt(e.Published),
		now(), e.ID)
	if err != nil {
		return fmt.Errorf("storage: update event: %w", err)
	}
	return requireAffected(res, "event")
}

func (s *EventStore) Delete(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM events WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("storage: delete event: %w", err)
	}
	return requireAffected(res, "event")
}

// List returns every event for the admin, newest first (undated events last).
func (s *EventStore) List(ctx context.Context) ([]Event, error) {
	return s.query(ctx, `SELECT `+eventColumns+` FROM events
		ORDER BY starts_at IS NULL, starts_at DESC, id DESC`)
}

// ListPublic returns published events of the given kind that have not yet
// ended as of now: featured first, then soonest start, undated last.
func (s *EventStore) ListPublic(ctx context.Context, kind string, at time.Time) ([]Event, error) {
	return s.query(ctx, `SELECT `+eventColumns+` FROM events
		WHERE published = 1 AND kind = ? AND (ends_at IS NULL OR ends_at >= ?)
		ORDER BY featured DESC, starts_at IS NULL, starts_at ASC, id ASC`, kind, formatTime(at))
}

func (s *EventStore) query(ctx context.Context, q string, args ...any) ([]Event, error) {
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("storage: list events: %w", err)
	}
	defer rows.Close()

	var events []Event
	for rows.Next() {
		e, err := scanEvent(rows)
		if err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, rows.Err()
}

type scanner interface{ Scan(dest ...any) error }

func scanEvent(r scanner) (Event, error) {
	var e Event
	var starts, ends sql.NullString
	var poster sql.NullInt64
	var featured, published int
	var created, updated string
	err := r.Scan(&e.ID, &e.Kind, &e.Title, &e.Summary, &e.BadgeText, &e.BadgeStyle, &starts, &ends, &poster,
		&e.LinkURL, &e.LinkLabel, &e.Link2URL, &e.Link2Label, &featured, &published, &created, &updated)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Event{}, err
		}
		return Event{}, fmt.Errorf("storage: scan event: %w", err)
	}
	e.Featured, e.Published = featured != 0, published != 0
	if poster.Valid {
		e.PosterImageID = &poster.Int64
	}
	if e.StartsAt, err = parseNullTime(starts); err != nil {
		return Event{}, err
	}
	if e.EndsAt, err = parseNullTime(ends); err != nil {
		return Event{}, err
	}
	if e.CreatedAt, err = parseTime(created); err != nil {
		return Event{}, fmt.Errorf("storage: parse event created_at: %w", err)
	}
	if e.UpdatedAt, err = parseTime(updated); err != nil {
		return Event{}, fmt.Errorf("storage: parse event updated_at: %w", err)
	}
	return e, nil
}

func nullTime(t *time.Time) any {
	if t == nil {
		return nil
	}
	return formatTime(*t)
}

func nullInt(i *int64) any {
	if i == nil {
		return nil
	}
	return *i
}

func parseNullTime(s sql.NullString) (*time.Time, error) {
	if !s.Valid {
		return nil, nil
	}
	t, err := parseTime(s.String)
	if err != nil {
		return nil, fmt.Errorf("storage: parse time %q: %w", s.String, err)
	}
	return &t, nil
}
