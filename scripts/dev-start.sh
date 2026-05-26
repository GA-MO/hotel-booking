#!/usr/bin/env bash
# Start API + worker + web apps in the background. PIDs land in .dev/*.pid
# and stdout/stderr in .dev/*.log so `make logs` can tail them.
set -Eeuo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
DEV_DIR="$ROOT/.dev"
mkdir -p "$DEV_DIR"

# Auto-export .env so the API + worker pick up DATABASE_URL etc. The web apps
# (Next.js) read .env.local / .env automatically, but Go's envconfig doesn't.
if [[ -f "$ROOT/.env" ]]; then
  set -a
  # shellcheck disable=SC1091
  source "$ROOT/.env"
  set +a
fi

is_running() {
  local pid_file="$1"
  [[ -f "$pid_file" ]] && kill -0 "$(cat "$pid_file")" 2>/dev/null
}

# `exec` inside the sh -c wrapper makes $! the long-running process PID
# (otherwise it'd be the now-dead wrapper shell).
start_bg() {
  local name="$1"
  local cmd="$2"
  local pid_file="$DEV_DIR/$name.pid"
  if is_running "$pid_file"; then
    echo "  $name already running (pid $(cat "$pid_file"))"
    return 0
  fi
  nohup sh -c "$cmd" >"$DEV_DIR/$name.log" 2>&1 &
  echo $! >"$pid_file"
  echo "  $name started (pid $!)"
}

start_bg api    "cd '$ROOT/apps/api' && exec go run ./cmd/api"
start_bg worker "cd '$ROOT/apps/api' && exec go run ./cmd/worker"
start_bg web    "cd '$ROOT' && exec pnpm dev"
