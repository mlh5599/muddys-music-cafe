package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"
)

// Staff kinds.
const (
	StaffKindDJ   = "dj"
	StaffKindHost = "host"
)

// StaffMember is one DJ or host. Bio paragraphs are separated by a blank line
// and Schedule lines by a newline; both are plain text.
type StaffMember struct {
	ID           int64
	Kind         string
	Slug         string
	Name         string
	SLName       string
	Bio          string
	Schedule     string
	PhotoImageID *int64
	SortOrder    int
	Active       bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// StaffStore persists staff_members.
type StaffStore struct{ db *sql.DB }

func NewStaffStore(db *sql.DB) *StaffStore { return &StaffStore{db: db} }

const staffColumns = `id, kind, slug, name, sl_name, bio, schedule, photo_image_id, sort_order, active, created_at, updated_at`

// Create inserts a staff member at the end of their list. If Slug is empty it
// is derived from the name and made unique within the kind.
func (s *StaffStore) Create(ctx context.Context, m StaffMember) (StaffMember, error) {
	if m.Slug == "" {
		slug, err := s.uniqueSlug(ctx, m.Kind, m.Name)
		if err != nil {
			return StaffMember{}, err
		}
		m.Slug = slug
	}
	order, err := nextSortOrder(ctx, s.db, "staff_members", "kind = ?", m.Kind)
	if err != nil {
		return StaffMember{}, err
	}
	if m.SortOrder == 0 {
		m.SortOrder = order
	}
	ts := now()
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO staff_members (kind, slug, name, sl_name, bio, schedule, photo_image_id, sort_order, active, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		m.Kind, m.Slug, m.Name, m.SLName, m.Bio, m.Schedule, nullInt(m.PhotoImageID), m.SortOrder, boolToInt(m.Active), ts, ts)
	if err != nil {
		return StaffMember{}, fmt.Errorf("storage: insert staff member: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return StaffMember{}, fmt.Errorf("storage: get staff member id: %w", err)
	}
	return s.Get(ctx, id)
}

func (s *StaffStore) Get(ctx context.Context, id int64) (StaffMember, error) {
	m, err := scanStaff(s.db.QueryRowContext(ctx, `SELECT `+staffColumns+` FROM staff_members WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return StaffMember{}, ErrNotFound
	}
	return m, err
}

// Update changes everything except kind, slug and sort_order (the slug stays
// stable so existing deep links keep working; use Reorder to reposition).
func (s *StaffStore) Update(ctx context.Context, m StaffMember) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE staff_members SET name = ?, sl_name = ?, bio = ?, schedule = ?, photo_image_id = ?, active = ?, updated_at = ?
		 WHERE id = ?`,
		m.Name, m.SLName, m.Bio, m.Schedule, nullInt(m.PhotoImageID), boolToInt(m.Active), now(), m.ID)
	if err != nil {
		return fmt.Errorf("storage: update staff member: %w", err)
	}
	return requireAffected(res, "staff member")
}

func (s *StaffStore) Delete(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM staff_members WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("storage: delete staff member: %w", err)
	}
	return requireAffected(res, "staff member")
}

// List returns staff of one kind in display order. Inactive (hidden) members
// are included only when includeInactive is true (the admin view).
func (s *StaffStore) List(ctx context.Context, kind string, includeInactive bool) ([]StaffMember, error) {
	q := `SELECT ` + staffColumns + ` FROM staff_members WHERE kind = ?`
	if !includeInactive {
		q += ` AND active = 1`
	}
	q += ` ORDER BY sort_order, id`
	rows, err := s.db.QueryContext(ctx, q, kind)
	if err != nil {
		return nil, fmt.Errorf("storage: list staff: %w", err)
	}
	defer rows.Close()

	var out []StaffMember
	for rows.Next() {
		m, err := scanStaff(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// Reorder sets the display order of the given staff ids (all of one kind).
func (s *StaffStore) Reorder(ctx context.Context, kind string, ids []int64) error {
	return reorder(ctx, s.db, "staff_members", "kind = ?", []any{kind}, ids)
}

func (s *StaffStore) uniqueSlug(ctx context.Context, kind, name string) (string, error) {
	base := slugify(name)
	if base == "" {
		base = kind
	}
	slug := base
	for n := 2; ; n++ {
		var exists int
		err := s.db.QueryRowContext(ctx, `SELECT 1 FROM staff_members WHERE kind = ? AND slug = ?`, kind, slug).Scan(&exists)
		if errors.Is(err, sql.ErrNoRows) {
			return slug, nil
		}
		if err != nil {
			return "", fmt.Errorf("storage: check slug: %w", err)
		}
		slug = fmt.Sprintf("%s-%d", base, n)
	}
}

// slugify lowercases name and joins its letter/digit runs with hyphens,
// dropping a leading "dj" word so "DJ Andy" becomes "andy".
func slugify(name string) string {
	var words []string
	var cur strings.Builder
	flush := func() {
		if cur.Len() > 0 {
			words = append(words, cur.String())
			cur.Reset()
		}
	}
	for _, r := range strings.ToLower(name) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			cur.WriteRune(r)
		} else {
			flush()
		}
	}
	flush()
	if len(words) > 1 && words[0] == "dj" {
		words = words[1:]
	}
	return strings.Join(words, "-")
}

func scanStaff(r scanner) (StaffMember, error) {
	var m StaffMember
	var photo sql.NullInt64
	var active int
	var created, updated string
	err := r.Scan(&m.ID, &m.Kind, &m.Slug, &m.Name, &m.SLName, &m.Bio, &m.Schedule, &photo, &m.SortOrder, &active, &created, &updated)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return StaffMember{}, err
		}
		return StaffMember{}, fmt.Errorf("storage: scan staff member: %w", err)
	}
	m.Active = active != 0
	if photo.Valid {
		m.PhotoImageID = &photo.Int64
	}
	if m.CreatedAt, err = parseTime(created); err != nil {
		return StaffMember{}, fmt.Errorf("storage: parse staff created_at: %w", err)
	}
	if m.UpdatedAt, err = parseTime(updated); err != nil {
		return StaffMember{}, fmt.Errorf("storage: parse staff updated_at: %w", err)
	}
	return m, nil
}
