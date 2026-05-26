#!/usr/bin/env bash
# Stop API + worker + web. SIGTERM first, escalate to SIGKILL after 5s.
# Also sweeps child processes (pnpm dev spawns next dev workers under it).
set -Eeuo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
DEV_DIR="$ROOT/.dev"
[[ -d "$DEV_DIR" ]] || { echo "nothing to stop (no .dev/)"; exit 0; }

stop_pid() {
  local name="$1" pid_file="$DEV_DIR/$name.pid"
  [[ -f "$pid_file" ]] || return 0
  local pid
  pid="$(cat "$pid_file")"
  if kill -0 "$pid" 2>/dev/null; then
    # Negative pgid kills the whole process group (so pnpm's child workers go too).
    local pgid
    pgid="$(ps -o pgid= -p "$pid" | tr -d ' ' || true)"
    if [[ -n "$pgid" ]]; then
      kill -TERM "-$pgid" 2>/dev/null || true
    else
      kill -TERM "$pid" 2>/dev/null || true
    fi
    for _ in $(seq 1 5); do
      kill -0 "$pid" 2>/dev/null || break
      sleep 1
    done
    if kill -0 "$pid" 2>/dev/null; then
      [[ -n "$pgid" ]] && kill -KILL "-$pgid" 2>/dev/null || kill -KILL "$pid" 2>/dev/null || true
    fi
    echo "  $name stopped (pid $pid)"
  else
    echo "  $name not running"
  fi
  rm -f "$pid_file"
}

stop_pid web
stop_pid worker
stop_pid api

# Belt-and-braces: catch stray next-server processes pnpm may have detached.
pkill -f "next-server" 2>/dev/null || true
pkill -f "next dev" 2>/dev/null || true
