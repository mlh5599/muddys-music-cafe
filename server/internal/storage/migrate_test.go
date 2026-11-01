package storage

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func rawDB(t *testing.T) (*sql.DB, string) {
	t.Helper()
	dir := t.TempDir()
	db, err := sql.Open("sqlite", "file:"+filepath.Join(dir, "m.db")+"?_pragma=journal_mode(WAL)")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db, filepath.Join(dir, "backups")
}

func version(t *testing.T, db *sql.DB) int {
	t.Helper()
	var v int
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&v); err != nil {
		t.Fatal(err)
	}
	return v
}

func backups(t *testing.T, dir string) []string {
	t.Helper()
	entries, _ := os.ReadDir(dir)
	var out []string
	for _, e := range entries {
		out = append(out, e.Name())
	}
	return out
}

var (
	m1 = migration{version: 1, name: "0001_a.sql", sql: `CREATE TABLE a (x TEXT)`}
	m2 = migration{version: 2, name: "0002_b.sql", sql: `ALTER TABLE a ADD COLUMN y TEXT`}
)

func TestMigrateFreshDatabaseNeedsNoBackup(t *testing.T) {
	db, bdir := rawDB(t)
	if err := applyMigrations(db, []migration{m1, m2}, bdir); err != nil {
		t.Fatal(err)
	}
	if v := version(t, db); v != 2 {
		t.Errorf("user_version = %d, want 2", v)
	}
	if got := backups(t, bdir); len(got) != 0 {
		t.Errorf("fresh database was backed up: %v", got)
	}
}

func TestMigrateBacksUpExistingDataFirst(t *testing.T) {
	db, bdir := rawDB(t)
	if err := applyMigrations(db, []migration{m1}, bdir); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO a (x) VALUES ('keep me')`); err != nil {
		t.Fatal(err)
	}

	if err := applyMigrations(db, []migration{m1, m2}, bdir); err != nil {
		t.Fatal(err)
	}
	got := backups(t, bdir)
	if len(got) != 1 || !strings.HasPrefix(got[0], "pre-v0002-") || !strings.HasSuffix(got[0], "-from-v0001.db") {
		t.Fatalf("backups = %v", got)
	}

	// The backup holds the pre-migration schema and data: no column y yet.
	bk, err := sql.Open("sqlite", "file:"+filepath.Join(bdir, got[0]))
	if err != nil {
		t.Fatal(err)
	}
	defer bk.Close()
	var x string
	if err := bk.QueryRow(`SELECT x FROM a`).Scan(&x); err != nil || x != "keep me" {
		t.Errorf("backup row = %q, %v", x, err)
	}
	if v := version(t, bk); v != 1 {
		t.Errorf("backup user_version = %d, want 1", v)
	}
	if _, err := bk.Exec(`SELECT y FROM a`); err == nil {
		t.Error("backup already has the migrated column")
	}
}

func TestMigrateUpToDateDoesNotBackUp(t *testing.T) {
	db, bdir := rawDB(t)
	if err := applyMigrations(db, []migration{m1, m2}, bdir); err != nil {
		t.Fatal(err)
	}
	if err := applyMigrations(db, []migration{m1, m2}, bdir); err != nil {
		t.Fatal(err)
	}
	if got := backups(t, bdir); len(got) != 0 {
		t.Errorf("no-op migrate made a backup: %v", got)
	}
}

func TestMigrateRefusesNewerDatabase(t *testing.T) {
	db, bdir := rawDB(t)
	if err := applyMigrations(db, []migration{m1, m2}, bdir); err != nil {
		t.Fatal(err)
	}
	err := applyMigrations(db, []migration{m1}, bdir)
	if err == nil || !strings.Contains(err.Error(), "newer than this build") {
		t.Fatalf("err = %v, want a newer-schema refusal", err)
	}
	if v := version(t, db); v != 2 {
		t.Errorf("refusal changed user_version to %d", v)
	}
}

func TestMigrateFailureLeavesVersionAndBackup(t *testing.T) {
	db, bdir := rawDB(t)
	if err := applyMigrations(db, []migration{m1}, bdir); err != nil {
		t.Fatal(err)
	}
	bad := migration{version: 2, name: "0002_bad.sql", sql: `ALTER TABLE nope ADD COLUMN z TEXT`}
	if err := applyMigrations(db, []migration{m1, bad}, bdir); err == nil {
		t.Fatal("bad migration succeeded")
	}
	if v := version(t, db); v != 1 {
		t.Errorf("user_version = %d after failed migration, want 1", v)
	}
	if len(backups(t, bdir)) != 1 {
		t.Errorf("expected the pre-migration backup to remain: %v", backups(t, bdir))
	}
}

func TestPruneBackupsKeepsNewest(t *testing.T) {
	dir := t.TempDir()
	for _, n := range []string{"pre-v0002-20260101T000000Z-from-v0001.db", "pre-v0002-20260102T000000Z-from-v0001.db",
		"pre-v0003-20260103T000000Z-from-v0002.db", "pre-v0003-20260104T000000Z-from-v0002.db",
		"pre-v0004-20260105T000000Z-from-v0003.db", "pre-v0004-20260106T000000Z-from-v0003.db", "notes.txt"} {
		os.WriteFile(filepath.Join(dir, n), nil, 0o600)
	}
	if err := pruneBackups(dir); err != nil {
		t.Fatal(err)
	}
	got := backups(t, dir)
	if len(got) != backupsToKeep+1 { // five backups plus the unrelated file
		t.Fatalf("after prune: %v", got)
	}
	for _, n := range got {
		if strings.Contains(n, "20260101") {
			t.Errorf("oldest backup survived: %v", got)
		}
	}
}

// Every frozen testdata/release-vN.sql stands in for a production database
// from that release. The current code must open each one and keep its data,
// so a migration that would break real data fails in CI rather than on deploy.
func TestCurrentCodeMigratesPreviousReleaseFixtures(t *testing.T) {
	fixtures, err := filepath.Glob(filepath.Join("testdata", "release-v*.sql"))
	if err != nil || len(fixtures) == 0 {
		t.Fatalf("no release fixtures found: %v", err)
	}
	for _, f := range fixtures {
		t.Run(filepath.Base(f), func(t *testing.T) {
			script, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(t.TempDir(), "muddys.db")
			old, err := sql.Open("sqlite", "file:"+path)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := old.Exec(string(script)); err != nil {
				t.Fatalf("load fixture: %v", err)
			}
			old.Close()

			db, err := Open(path)
			if err != nil {
				t.Fatalf("current code cannot open this release's database: %v", err)
			}
			defer db.Close()
			var title string
			if err := db.QueryRow(`SELECT title FROM events WHERE id = 1`).Scan(&title); err != nil || title != "Fixture event" {
				t.Errorf("fixture data lost: %q, %v", title, err)
			}
			var url string
			if err := db.QueryRow(`SELECT value FROM settings WHERE key = 'radio_stream_url'`).Scan(&url); err != nil || url == "" {
				t.Errorf("setting lost: %q, %v", url, err)
			}
		})
	}
}
