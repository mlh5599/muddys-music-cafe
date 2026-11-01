package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Partner is one business partner shown on the home page.
type Partner struct {
	ID        int64
	Name      string
	SortOrder int
	Active    bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

// PartnerStore persists partners.
type PartnerStore struct{ db *sql.DB }

func NewPartnerStore(db *sql.DB) *PartnerStore { return &PartnerStore{db: db} }

const partnerColumns = `id, name, sort_order, active, created_at, updated_at`

// Create inserts a partner at the end of the list.
func (s *PartnerStore) Create(ctx context.Context, p Partner) (Partner, error) {
	order, err := nextSortOrder(ctx, s.db, "partners", "1 = 1")
	if err != nil {
		return Partner{}, err
	}
	if p.SortOrder == 0 {
		p.SortOrder = order
	}
	ts := now()
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO partners (name, sort_order, active, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
		p.Name, p.SortOrder, boolToInt(p.Active), ts, ts)
	if err != nil {
		return Partner{}, fmt.Errorf("storage: insert partner: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Partner{}, fmt.Errorf("storage: get partner id: %w", err)
	}
	return s.Get(ctx, id)
}

func (s *PartnerStore) Get(ctx context.Context, id int64) (Partner, error) {
	p, err := scanPartner(s.db.QueryRowContext(ctx, `SELECT `+partnerColumns+` FROM partners WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Partner{}, ErrNotFound
	}
	return p, err
}

func (s *PartnerStore) Update(ctx context.Context, p Partner) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE partners SET name = ?, active = ?, updated_at = ? WHERE id = ?`,
		p.Name, boolToInt(p.Active), now(), p.ID)
	if err != nil {
		return fmt.Errorf("storage: update partner: %w", err)
	}
	return requireAffected(res, "partner")
}

func (s *PartnerStore) Delete(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM partners WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("storage: delete partner: %w", err)
	}
	return requireAffected(res, "partner")
}

// List returns partners in display order; hidden ones only if includeInactive.
func (s *PartnerStore) List(ctx context.Context, includeInactive bool) ([]Partner, error) {
	q := `SELECT ` + partnerColumns + ` FROM partners`
	if !includeInactive {
		q += ` WHERE active = 1`
	}
	q += ` ORDER BY sort_order, id`
	rows, err := s.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("storage: list partners: %w", err)
	}
	defer rows.Close()

	var out []Partner
	for rows.Next() {
		p, err := scanPartner(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// Reorder sets the display order of the given partner ids.
func (s *PartnerStore) Reorder(ctx context.Context, ids []int64) error {
	return reorder(ctx, s.db, "partners", "1 = 1", nil, ids)
}

func scanPartner(r scanner) (Partner, error) {
	var p Partner
	var active int
	var created, updated string
	err := r.Scan(&p.ID, &p.Name, &p.SortOrder, &active, &created, &updated)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Partner{}, err
		}
		return Partner{}, fmt.Errorf("storage: scan partner: %w", err)
	}
	p.Active = active != 0
	if p.CreatedAt, err = parseTime(created); err != nil {
		return Partner{}, fmt.Errorf("storage: parse partner created_at: %w", err)
	}
	if p.UpdatedAt, err = parseTime(updated); err != nil {
		return Partner{}, fmt.Errorf("storage: parse partner updated_at: %w", err)
	}
	return p, nil
}
