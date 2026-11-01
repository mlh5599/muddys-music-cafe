package storage

import (
	"database/sql"
	"embed"
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

type migration struct {
	version int
	name    string
	sql     string
}

// loadMigrations reads every embedded *.sql file and orders them by the
// numeric prefix of their filename (e.g. "0001_init.sql" -> version 1).
func loadMigrations() ([]migration, error) {
	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return nil, fmt.Errorf("storage: read migrations dir: %w", err)
	}

	migrations := make([]migration, 0, len(entries))
	for _, e := range entries {
		name := e.Name()
		prefix, _, ok := strings.Cut(name, "_")
		if !ok {
			return nil, fmt.Errorf("storage: migration file %q missing version prefix", name)
		}
		version, err := strconv.Atoi(prefix)
		if err != nil {
			return nil, fmt.Errorf("storage: migration file %q has non-numeric version prefix: %w", name, err)
		}
		contents, err := migrationsFS.ReadFile(path.Join("migrations", name))
		if err != nil {
			return nil, fmt.Errorf("storage: read migration %q: %w", name, err)
		}
		migrations = append(migrations, migration{version: version, name: name, sql: string(contents)})
	}

	sort.Slice(migrations, func(i, j int) bool { return migrations[i].version < migrations[j].version })
	for i, m := range migrations {
		if m.version != i+1 {
			return nil, fmt.Errorf("storage: migrations must be a gapless sequence starting at 1, got version %d at position %d", m.version, i+1)
		}
	}
	return migrations, nil
}

// backupsToKeep is how many pre-migration backups are retained.
const backupsToKeep = 5

// migrate applies the embedded migrations. backupDir receives a copy of the
// database before any pending migration runs against existing data.
func migrate(db *sql.DB, backupDir string) error {
	migrations, err := loadMigrations()
	if err != nil {
		return err
	}
	return applyMigrations(db, migrations, backupDir)
}

// applyMigrations applies every migration newer than the database's current
// PRAGMA user_version, each inside its own transaction with user_version
// updated as the last statement, so a failed or interrupted migration never
// leaves the schema and the version marker disagreeing.
//
// Two safeguards sit around that:
//   - If the database is newer than this build (an older release was started
//     against a newer schema), it refuses rather than run code against a
//     schema it does not understand.
//   - Before the first pending migration touches a database that already has
//     data, it writes a backup, so a bad migration or release can be undone.
func applyMigrations(db *sql.DB, migrations []migration, backupDir string) error {
	var current int
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&current); err != nil {
		return fmt.Errorf("storage: read user_version: %w", err)
	}

	latest := len(migrations)
	if current > latest {
		return fmt.Errorf("storage: database schema version %d is newer than this build supports (%d); refusing to start. Run the release that created it, or restore a backup from %s", current, latest, backupDir)
	}
	if current == latest {
		return nil
	}

	// current == 0 is a brand-new database: nothing to protect.
	if current > 0 {
		if err := backupBeforeMigrating(db, backupDir, current, latest); err != nil {
			return err
		}
	}

	for _, m := range migrations {
		if m.version <= current {
			continue
		}

		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("storage: begin migration %q: %w", m.name, err)
		}
		if _, err := tx.Exec(m.sql); err != nil {
			tx.Rollback()
			return fmt.Errorf("storage: apply migration %q: %w", m.name, err)
		}
		// user_version can't be a bind parameter; m.version comes from our
		// own embedded filenames, not user input.
		if _, err := tx.Exec(fmt.Sprintf(`PRAGMA user_version = %d`, m.version)); err != nil {
			tx.Rollback()
			return fmt.Errorf("storage: set user_version after migration %q: %w", m.name, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("storage: commit migration %q: %w", m.name, err)
		}
	}
	return nil
}

// backupBeforeMigrating copies the database to
// <backupDir>/pre-v<to>-<UTC timestamp>-from-v<from>.db with VACUUM INTO (a
// consistent copy, including anything still in the WAL) and prunes old ones.
func backupBeforeMigrating(db *sql.DB, backupDir string, from, to int) error {
	if err := os.MkdirAll(backupDir, 0o750); err != nil {
		return fmt.Errorf("storage: create backup dir: %w", err)
	}
	name := fmt.Sprintf("pre-v%04d-%s-from-v%04d.db", to, time.Now().UTC().Format("20060102T150405Z"), from)
	dest := filepath.Join(backupDir, name)
	// VACUUM INTO takes a string literal, not a bind parameter.
	if _, err := db.Exec(`VACUUM INTO '` + strings.ReplaceAll(dest, `'`, `''`) + `'`); err != nil {
		return fmt.Errorf("storage: back up database before migrating: %w", err)
	}
	log.Printf("storage: backed up database (schema v%d) to %s before migrating to v%d", from, dest, to)
	return pruneBackups(backupDir)
}

func pruneBackups(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("storage: list backups: %w", err)
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasPrefix(e.Name(), "pre-v") && strings.HasSuffix(e.Name(), ".db") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names) // timestamp in the name makes oldest sort first
	for len(names) > backupsToKeep {
		if err := os.Remove(filepath.Join(dir, names[0])); err != nil {
			return fmt.Errorf("storage: prune backup: %w", err)
		}
		names = names[1:]
	}
	return nil
}
