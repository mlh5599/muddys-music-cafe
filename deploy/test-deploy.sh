#!/bin/sh
# Tests deploy.sh with stub docker/cosign commands. Run: deploy/test-deploy.sh
set -eu
here=$(cd "$(dirname "$0")" && pwd)
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
fail=0

D1=sha256:1111111111111111111111111111111111111111111111111111111111111111
D2=sha256:2222222222222222222222222222222222222222222222222222222222222222
REPO=ghcr.io/mlh5599/muddys-music-cafe

# Stubs: docker records calls; "health" file decides the reported health, and
# "unhealthy_image" names an image that never becomes healthy. A compose "up"
# of an image that is configured to migrate drops a backup, like the real app.
mkdir -p "$work/bin"
cat > "$work/bin/cosign" <<'S'
#!/bin/sh
[ -f "$STUB_DIR/cosign_fail" ] && { echo "signature invalid" >&2; exit 1; }
echo "cosign $*" >> "$STUB_DIR/calls"
S
cat > "$work/bin/docker" <<'S'
#!/bin/sh
echo "docker $*" >> "$STUB_DIR/calls"
case "$1" in
  pull) ;;
  inspect)
    img=$(sed -n 's/^MUDDYS_IMAGE=//p' "$MUDDYS_DEPLOY_DIR/.deploy.env")
    if [ "$img" = "$(cat "$STUB_DIR/unhealthy_image" 2>/dev/null)" ]; then echo unhealthy; else echo healthy; fi ;;
  compose)
    shift; [ "$1" = "--env-file" ] && shift 2
    case "$1" in
      ps) echo cid123 ;;
      logs) echo "app log line" ;;
      stop) echo stopped >> "$STUB_DIR/calls" ;;
      up)
        img=$(sed -n 's/^MUDDYS_IMAGE=//p' "$MUDDYS_DEPLOY_DIR/.deploy.env")
        if [ -f "$STUB_DIR/migrates_image" ] && [ "$img" = "$(cat "$STUB_DIR/migrates_image")" ]; then
          mkdir -p "$MUDDYS_DATA_DIR/backups"
          cp "$MUDDYS_DATA_DIR/muddys.db" "$MUDDYS_DATA_DIR/backups/pre-v0002-20260101T000000Z-from-v0001.db"
          echo "migrated-db" > "$MUDDYS_DATA_DIR/muddys.db"
        fi ;;
    esac ;;
esac
S
chmod +x "$work/bin/"*

run() { # run TAG DIGEST -> sets $out, $rc
  rc=0
  out=$(PATH="$work/bin:$PATH" STUB_DIR="$work" MUDDYS_DEPLOY_DIR="$work/app" MUDDYS_DATA_DIR="$work/app/data" \
    MUDDYS_HEALTH_WAIT=5 "$here/deploy.sh" "$@" 2>&1) || rc=$?
}
reset() {
  rm -rf "$work/app" "$work/calls" "$work/unhealthy_image" "$work/migrates_image" "$work/cosign_fail"
  mkdir -p "$work/app/data"
  echo "original-db" > "$work/app/data/muddys.db"
  printf 'MUDDYS_IMAGE=%s@%s\n' "$REPO" "$D1" > "$work/app/.deploy.env"
}
check() { # check NAME CONDITION...
  name=$1; shift
  if "$@"; then echo "ok   $name"; else echo "FAIL $name"; echo "$out"; fail=1; fi
}
current_image() { sed -n 's/^MUDDYS_IMAGE=//p' "$work/app/.deploy.env"; }
db() { cat "$work/app/data/muddys.db"; }

# sleep is stubbed out via PATH so the script's waits cost nothing
cat > "$work/bin/sleep" <<'S'
#!/bin/sh
exit 0
S
chmod +x "$work/bin/sleep"

reset; run v2026.11.01.2 "$D2"
check "healthy deploy succeeds" test $rc -eq 0
check "healthy deploy switches image" test "$(current_image)" = "$REPO@$D2"
check "signature verified before pull" sh -c "grep -n 'cosign\|docker pull' '$work/calls' | head -1 | grep -q cosign"

reset; run v2026.11.01.1 "$D1"
check "redeploying the running digest is a no-op" test $rc -eq 0
check "no-op does not pull" sh -c "! grep -q 'docker pull' '$work/calls'"

reset; echo "$REPO@$D2" > "$work/unhealthy_image"; run v2026.11.01.2 "$D2"
check "unhealthy deploy fails" test $rc -eq 1
check "unhealthy deploy rolls back the image" test "$(current_image)" = "$REPO@$D1"
check "no migration -> database untouched" test "$(db)" = original-db

reset; echo "$REPO@$D2" > "$work/unhealthy_image"; echo "$REPO@$D2" > "$work/migrates_image"; run v2026.11.01.2 "$D2"
check "failed deploy after migration fails" test $rc -eq 1
check "migration rollback restores the database" test "$(db)" = original-db
check "migration rollback restores the image" test "$(current_image)" = "$REPO@$D1"

reset; touch "$work/cosign_fail"; run v2026.11.01.2 "$D2"
check "bad signature aborts" test $rc -ne 0
check "bad signature never pulls" sh -c "! grep -q 'docker pull' '$work/calls'"
check "bad signature leaves the image alone" test "$(current_image)" = "$REPO@$D1"

reset; run 'v1;rm' "$D2"
check "bad tag rejected" test $rc -eq 2
reset; run v2026.11.01.2 "sha256:abc"
check "bad digest rejected" test $rc -eq 2

exit $fail
