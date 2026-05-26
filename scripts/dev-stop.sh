#!/usr/bin/env bash
# Stop API + worker + web. SIGTERM first, escalate to SIGKILL after 5s.
# Also sweeps the pgid so pnpm dev's spawned next workers die with it.
set -Eeuo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
DEV_DIR="$ROOT/.dev"
[[ -d "$DEV_DIR" ]] || { echo "nothing to stop (no .dev/)"; exit 0; }

stop_pid() {
  local name="$1"
  local pid_file="$DEV_DIR/$name.pid"
  [[ -f "$pid_file" ]] || return 0
  local pid
  pid="$(cat "$pid_file")"
  if kill -0 "$pid" 2>/dev/null; then
    local pgid
    pgid="$(ps -o pgid= -p "$pid" | tr -d ' ' 2>/dev/null || true)"
    if [[ -n "$pgid" ]]; then
      kill -TERM "-$pgid" 2>/dev/null || true
    else
      kill -TERM "$pid" 2>/dev/null || true
    fi
    local i
    for i in 1 2 3 4 5; do
      kill -0 "$pid" 2>/dev/null || break
      sleep 1
    done
    if kill -0 "$pid" 2>/dev/null; then
      if [[ -n "$pgid" ]]; then
        kill -KILL "-$pgid" 2>/dev/null || true
      else
        kill -KILL "$pid" 2>/dev/null || true
      fi
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

# Belt-and-braces: catch any stray next-server processes pnpm may have detached.
pkill -f "next-server" 2>/dev/null || true
pkill -f "next dev" 2>/dev/null || true
