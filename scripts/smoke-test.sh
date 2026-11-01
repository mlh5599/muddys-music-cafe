#!/bin/sh
# Smoke test a built image: seed an empty data volume, start the container,
# wait for its HEALTHCHECK to pass, and fetch the public pages.
# Usage: scripts/smoke-test.sh IMAGE
set -eu

image=${1:?usage: smoke-test.sh IMAGE}
name=muddys-smoke-$$
vol=$name-data

cleanup() {
  docker rm -f "$name" >/dev/null 2>&1 || true
  docker volume rm "$vol" >/dev/null 2>&1 || true
}
trap cleanup EXIT

docker volume create "$vol" >/dev/null
# A fresh volume is root-owned; the image runs as uid 1000.
docker run --rm -v "$vol":/data --user 0 --entrypoint chown "$image" -R 1000:1000 /data
docker run --rm -v "$vol":/data --entrypoint /usr/local/bin/muddys "$image" -seed -data-dir=/data

docker run -d --name "$name" -v "$vol":/data -p 127.0.0.1::8080 "$image" >/dev/null

echo "waiting for the container to report healthy..."
i=0
while :; do
  status=$(docker inspect --format '{{.State.Health.Status}}' "$name")
  [ "$status" = healthy ] && break
  if [ "$status" = unhealthy ] || [ "$(docker inspect --format '{{.State.Running}}' "$name")" != true ] || [ $i -ge 60 ]; then
    echo "FAIL: container is $status after ${i}s" >&2
    docker logs "$name" >&2
    exit 1
  fi
  i=$((i + 1))
  sleep 1
done

fail() { echo "FAIL: $*" >&2; docker logs "$name" >&2; exit 1; }
port=$(docker port "$name" 8080/tcp | head -n1 | sed "s/.*://")
base=http://127.0.0.1:$port

[ "$(curl -s -o /dev/null -w '%{http_code}' "$base/healthz")" = 200 ] || fail "/healthz is not 200"
home=$(curl -fsS "$base/") || fail "home page did not load"
echo "$home" | grep -q 'data-stream="/radio/stream"' || fail "home page does not point the player at the relay"
for p in /djs /hosts; do
  [ "$(curl -s -o /dev/null -w '%{http_code}' "$base$p")" = 200 ] || fail "$p is not 200"
done
echo "smoke test passed: $image"
