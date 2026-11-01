#!/bin/sh
# Tests rollback-db.sh. Run: deploy/test-rollback-db.sh
set -eu
here=$(cd "$(dirname "$0")" && pwd)
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
fail=0
check() { if "$@"; then echo "ok   $name"; else echo "FAIL $name"; fail=1; fi; }

d=$work/data; mkdir -p "$d/backups"
echo old > "$d/backups/pre-v0001-old.db"
touch -d '2 hours ago' "$d/backups/pre-v0001-old.db"
marker=$work/marker; : > "$marker"

name="no new backup leaves the database alone"
echo current > "$d/muddys.db"
HOOK_DATA_DIR=$d "$here/rollback-db.sh" "$marker" >/dev/null
check test "$(cat "$d/muddys.db")" = current

name="backup made after the marker is restored"
sleep 1; echo original > "$d/backups/pre-v0002-new.db"
echo migrated > "$d/muddys.db"; echo wal > "$d/muddys.db-wal"; echo shm > "$d/muddys.db-shm"
HOOK_DATA_DIR=$d "$here/rollback-db.sh" "$marker" >/dev/null
check test "$(cat "$d/muddys.db")" = original
name="stale wal and shm are removed"
check test ! -e "$d/muddys.db-wal" -a ! -e "$d/muddys.db-shm"

name="missing backups directory is not an error"
rm -rf "$d/backups"
HOOK_DATA_DIR=$d "$here/rollback-db.sh" "$marker" >/dev/null
check true

exit $fail
