#!/bin/sh
# Rollback hook for deploy-receiver (see its ROLLBACK_HOOK): run as
#   rollback-db.sh MARKER_FILE
# after a failed release has been stopped and before the previous one starts.
#
# The app takes a backup (data/backups/pre-v*.db) before applying migrations.
# A backup newer than the marker, which was created when this deploy began,
# means the failed release was about to migrate the database, or did, so put the
# database back before the older image touches it.
#
# Environment: HOOK_DATA_DIR = the app's data directory.
set -eu
marker=${1:?usage: rollback-db.sh MARKER_FILE}
data=${HOOK_DATA_DIR:?HOOK_DATA_DIR is not set}
db=$data/muddys.db

[ -d "$data/backups" ] || { echo "no backups directory; database left as is"; exit 0; }
backup=$(find "$data/backups" -name 'pre-v*.db' -newer "$marker" | sort | tail -n 1)
if [ -z "$backup" ]; then
  echo "no migration ran during this deploy; database left as is"
  exit 0
fi

echo "restoring database from $backup"
cp "$backup" "$db.restore"
rm -f "$db-wal" "$db-shm"
mv "$db.restore" "$db"
