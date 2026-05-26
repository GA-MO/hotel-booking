#!/usr/bin/env bash
# Start API + worker + web apps in the background. PIDs land in .dev/*.pid
# and stdout/stderr land in .dev/*.log so `make logs` can tail them.
set -Eeuo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
DEV_DIR="$ROOT/.dev"
mkdir -p "$DEV_DIR"

# Idempotency: don't double-start.
start_if_missing() {
  local name="$1" pid_file="$DEV_DIR/$name.pid"
  if [[ -f "$pid_file" ]] && kill -0 "$(cat "$pid_file")" 2>/dev/null; then
    echo "  $name already running (pid $(cat "$pid_file"))"
    return 1
  fi
  return 0
}

if start_if_missing api; then
  (cd "$ROOT/apps/api" && nohup go run ./cmd/api > "$DEV_DIR/api.log" 2>&1 &)
  echo $! > "$DEV_DIR/api.pid"
  echo "  api started (pid $!)"
fi

if start_if_missing worker; then
  (cd "$ROOT/apps/api" && nohup go run ./cmd/worker > "$DEV_DIR/worker.log" 2>&1 &)
  echo $! > "$DEV_DIR/worker.pid"
  echo "  worker started (pid $!)"
fi

if start_if_missing web; then
  (cd "$ROOT" && nohup pnpm dev > "$DEV_DIR/web.log" 2>&1 &)
  echo $! > "$DEV_DIR/web.pid"
  echo "  web started (pid $!)"
fi
