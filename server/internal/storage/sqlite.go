// Package storage owns the SQLite schema, connection setup and the stores
// that read and write site content: WAL mode, busy-timeout, foreign keys and
// versioned migrations.
package storage

import (
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// ErrNotFound is returned when a requested row does not exist.
var ErrNotFound = errors.New("storage: not found")

// Open opens (creating if necessary) the SQLite database at path, configures
// it for WAL journaling, a busy timeout and foreign key enforcement, and
// applies any pending migrations. Before migrating existing data it backs the
// database up to a backups directory next to it, and it refuses to open a
// database whose schema is newer than this build.
//
// path is passed through to SQLite's own URI-filename parsing
// (https://sqlite.org/uri.html), which accepts plain relative and absolute
// filesystem paths.
func Open(path string) (*sql.DB, error) {
	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("storage: open %s: %w", path, err)
	}

	if err := migrate(db, filepath.Join(filepath.Dir(path), "backups")); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

// now returns the current time as stored in the database: RFC3339, UTC.
func now() string { return formatTime(time.Now()) }

func formatTime(t time.Time) string { return t.UTC().Format(time.RFC3339) }

func parseTime(s string) (time.Time, error) { return time.Parse(time.RFC3339, s) }

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
