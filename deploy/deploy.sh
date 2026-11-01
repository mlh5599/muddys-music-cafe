#!/bin/sh
# Deploy one signed release image and roll back if it does not come up healthy.
# Usage: deploy.sh TAG DIGEST
#
# Run on the docker host by the deploy receiver. Configuration comes from the
# environment (set by the service unit), never from the caller:
#   MUDDYS_DEPLOY_DIR   directory holding docker-compose.yml, .deploy.env, data
#   MUDDYS_SERVICE      compose service name (default: muddys)
#   MUDDYS_DATA_DIR     data directory (default: $MUDDYS_DEPLOY_DIR/Volumes/muddys/data)
#   MUDDYS_HEALTH_WAIT  seconds to wait for healthy (default: 120)
# The image repository and the signing identity are fixed below.
set -eu

IMAGE_REPO=ghcr.io/mlh5599/muddys-music-cafe
SIGNER_RE='^https://github\.com/mlh5599/muddys-music-cafe/\.github/workflows/release\.yml@refs/heads/main$'
OIDC_ISSUER=https://token.actions.githubusercontent.com

tag=${1:?usage: deploy.sh TAG DIGEST}
digest=${2:?usage: deploy.sh TAG DIGEST}
dir=${MUDDYS_DEPLOY_DIR:?MUDDYS_DEPLOY_DIR is not set}
svc=${MUDDYS_SERVICE:-muddys}
data=${MUDDYS_DATA_DIR:-$dir/Volumes/muddys/data}
wait_s=${MUDDYS_HEALTH_WAIT:-120}

# Re-validate: this script is the last line of defence.
printf '%s' "$tag" | grep -Eq '^v[0-9]{4}\.[0-9]{2}\.[0-9]{2}\.[0-9]+$' || { echo "bad tag" >&2; exit 2; }
printf '%s' "$digest" | grep -Eq '^sha256:[0-9a-f]{64}$' || { echo "bad digest" >&2; exit 2; }

ref=$IMAGE_REPO@$digest
envfile=$dir/.deploy.env
cd "$dir"

compose() { docker compose --env-file "$envfile" "$@"; }

healthy() {
  cid=$(compose ps -q "$svc" 2>/dev/null || true)
  [ -n "$cid" ] && [ "$(docker inspect --format '{{.State.Health.Status}}' "$cid" 2>/dev/null)" = healthy ]
}

wait_healthy() {
  i=0
  while [ "$i" -lt "$wait_s" ]; do
    healthy && return 0
    i=$((i + 5))
    sleep 5
  done
  return 1
}

current=
[ -f "$envfile" ] && current=$(sed -n 's/^MUDDYS_IMAGE=//p' "$envfile")

if [ "$current" = "$ref" ] && healthy; then
  echo "$tag is already running and healthy; nothing to do"
  exit 0
fi

echo "== verifying signature of $ref"
cosign verify \
  --certificate-identity-regexp "$SIGNER_RE" \
  --certificate-oidc-issuer "$OIDC_ISSUER" \
  "$ref" >/dev/null

echo "== pulling $ref"
docker pull "$ref"

marker=$dir/.deploy-started
: > "$marker"

echo "== starting $tag"
printf 'MUDDYS_IMAGE=%s\n' "$ref" > "$envfile.new"
mv "$envfile.new" "$envfile"
compose up -d --no-build "$svc"

if wait_healthy; then
  echo "== $tag is healthy"
  rm -f "$marker"
  exit 0
fi

echo "== $tag did not become healthy within ${wait_s}s; rolling back" >&2
compose logs --tail 50 "$svc" >&2 || true

if [ -z "$current" ]; then
  echo "no previous release recorded; leaving the failed one stopped" >&2
  compose stop "$svc" || true
  exit 1
fi

# A backup newer than the start marker means a migration was about to run (or
# did), so put the database back before the older image touches it.
backup=
if [ -d "$data/backups" ]; then
  backup=$(find "$data/backups" -name 'pre-v*.db' -newer "$marker" | sort | tail -n 1)
fi

compose stop "$svc" || true
if [ -n "$backup" ]; then
  echo "== restoring database from $backup"
  cp "$backup" "$data/muddys.db.restore"
  rm -f "$data/muddys.db-wal" "$data/muddys.db-shm"
  mv "$data/muddys.db.restore" "$data/muddys.db"
fi

printf 'MUDDYS_IMAGE=%s\n' "$current" > "$envfile.new"
mv "$envfile.new" "$envfile"
compose up -d --no-build "$svc"
if wait_healthy; then
  echo "== rolled back to the previous release; it is healthy" >&2
else
  echo "== ROLLBACK ALSO FAILED: manual intervention needed" >&2
fi
rm -f "$marker"
exit 1
