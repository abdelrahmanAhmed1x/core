#!/usr/bin/env bash
# Runs only against a new disposable container. Never flushes existing stores.
set -euo pipefail
image=${1:-redis:7.4.2-alpine}
container="core-integration-${RANDOM}-$$"
args=()
if [[ "$image" == *dragonfly* ]]; then args=(--logtostderr --proactor_threads=2); fi
trap 'docker rm -f "$container" >/dev/null 2>&1 || true' EXIT
docker run -d --name "$container" -p 127.0.0.1::6379 "$image" "${args[@]}" >/dev/null
port=$(docker port "$container" 6379/tcp | head -n1 | cut -d: -f2)
export CORE_INTEGRATION=1 CORE_REDIS_ADDR="127.0.0.1:$port"
for attempt in {1..30}; do
  if docker exec "$container" sh -c 'redis-cli ping' >/dev/null 2>&1; then break; fi
  # Dragonfly images may not include redis-cli; Go tests perform the final ping.
  sleep 1
  if [[ "$image" == *dragonfly* && "$attempt" -ge 3 ]]; then break; fi
done
go test -race -count=1 ./...
docker restart "$container" >/dev/null
# Docker may allocate a different ephemeral host port on restart.
port=$(docker port "$container" 6379/tcp | head -n1 | cut -d: -f2)
export CORE_REDIS_ADDR="127.0.0.1:$port"
sleep 2
go test -count=1 ./auth/... ./cache/... ./ratelimiter/...
