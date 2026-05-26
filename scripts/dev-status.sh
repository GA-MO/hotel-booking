#!/usr/bin/env bash
# Show which dev processes are alive.
set -Eeuo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
DEV_DIR="$ROOT/.dev"

check() {
  local name="$1" port="$2" pid_file="$DEV_DIR/$name.pid"
  if [[ -f "$pid_file" ]] && kill -0 "$(cat "$pid_file")" 2>/dev/null; then
    printf "  %-8s ✓ running (pid %s) — http://localhost:%s\n" "$name" "$(cat "$pid_file")" "$port"
  else
    printf "  %-8s — stopped\n" "$name"
  fi
}

echo "▸ dev processes"
check api    8080
check worker -
check web    3000

echo ""
echo "▸ docker infra"
docker compose ps --format 'table {{.Service}}\t{{.Status}}\t{{.Ports}}' 2>/dev/null || echo "  (docker not running)"
