package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Image is one uploaded file.
type Image struct {
	ID          int64
	Filename    string
	ContentType string
	SizeBytes   int64
	Width       int
	Height      int
	CreatedAt   time.Time
}

// ImageStore persists the images table.
type ImageStore struct{ db *sql.DB }

func NewImageStore(db *sql.DB) *ImageStore { return &ImageStore{db: db} }

func (s *ImageStore) Create(ctx context.Context, img Image) (Image, error) {
	created := time.Now().UTC().Truncate(time.Second)
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO images (filename, content_type, size_bytes, width, height, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		img.Filename, img.ContentType, img.SizeBytes, img.Width, img.Height, formatTime(created))
	if err != nil {
		return Image{}, fmt.Errorf("storage: insert image: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Image{}, fmt.Errorf("storage: get image id: %w", err)
	}
	img.ID, img.CreatedAt = id, created
	return img, nil
}

func (s *ImageStore) Get(ctx context.Context, id int64) (Image, error) {
	var img Image
	var created string
	err := s.db.QueryRowContext(ctx,
		`SELECT id, filename, content_type, size_bytes, width, height, created_at FROM images WHERE id = ?`, id).
		Scan(&img.ID, &img.Filename, &img.ContentType, &img.SizeBytes, &img.Width, &img.Height, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return Image{}, ErrNotFound
	}
	if err != nil {
		return Image{}, fmt.Errorf("storage: get image: %w", err)
	}
	if img.CreatedAt, err = parseTime(created); err != nil {
		return Image{}, fmt.Errorf("storage: parse image created_at: %w", err)
	}
	return img, nil
}

// Delete removes the image row. Anything referencing it (event posters, staff
// photos) has its reference cleared by the foreign key's ON DELETE SET NULL.
// The caller is responsible for removing the file from disk.
func (s *ImageStore) Delete(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM images WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("storage: delete image: %w", err)
	}
	return requireAffected(res, "image")
}

// requireAffected returns ErrNotFound if the statement changed no rows.
func requireAffected(res sql.Result, what string) error {
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("storage: rows affected (%s): %w", what, err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
