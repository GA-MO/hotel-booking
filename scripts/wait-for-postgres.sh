#!/usr/bin/env bash
# Block until the dev Postgres container accepts queries. Times out at 30s.
set -Eeuo pipefail

for i in $(seq 1 30); do
  if docker compose exec -T postgres pg_isready -U hotel >/dev/null 2>&1; then
    exit 0
  fi
  sleep 1
done

echo "postgres did not become ready within 30s" >&2
exit 1
