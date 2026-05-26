#!/usr/bin/env bash
# ============================================================================
# backup-postgres.sh — nightly Postgres dump → Backblaze B2
# ============================================================================
#
# Choice note: we use the official `b2` CLI (not rclone) because it has
# first-class support for B2 application keys, lifecycle rules, and bucket
# operations and is the lowest-friction option for B2-only off-site backups.
#
# ---- Setup (one time, on the production host) ------------------------------
#
# 1. Install the b2 CLI (Ubuntu 24.04 — pipx avoids polluting system pip):
#       sudo apt-get install -y pipx
#       pipx install b2
#       pipx ensurepath
#
# 2. Create a B2 application key scoped to a single bucket
#    (https://secure.backblaze.com/app_keys.htm) and authorize:
#       b2 account authorize "$B2_APPLICATION_KEY_ID" "$B2_APPLICATION_KEY"
#    The CLI caches credentials in ~/.b2_account_info — keep that file 0600.
#
# 3. Create the bucket if it doesn't exist (private, with lifecycle = none —
#    this script handles retention itself):
#       b2 bucket create hotel-booking-backups allPrivate
#
# 4. Schedule via cron as the deploy user (NOT root):
#       crontab -e
#       0 2 * * * /opt/hotel-booking/infra/backup/backup-postgres.sh >> /var/log/hotel-backup.log 2>&1
#
# 5. Verify weekly that the latest backup restores cleanly. See infra/README.md
#    "Backup verification".
#
# ---- Required env ----------------------------------------------------------
#   B2_BUCKET                e.g. hotel-booking-backups
#   POSTGRES_CONTAINER       docker compose service name, default "postgres"
#   POSTGRES_USER            default "hotel"
#   POSTGRES_DB              default "hotel_booking"
#   COMPOSE_FILE             default /opt/hotel-booking/docker-compose.prod.yml
#   COMPOSE_ENV_FILE         default /opt/hotel-booking/.env.prod
# ============================================================================

set -Eeuo pipefail

log()  { printf '[%s] %s\n' "$(date -u +'%Y-%m-%dT%H:%M:%SZ')" "$*"; }
fail() { log "ERROR: $*" >&2; exit 1; }

# ---- Config ----------------------------------------------------------------
B2_BUCKET="${B2_BUCKET:?B2_BUCKET must be set}"
POSTGRES_CONTAINER="${POSTGRES_CONTAINER:-postgres}"
POSTGRES_USER="${POSTGRES_USER:-hotel}"
POSTGRES_DB="${POSTGRES_DB:-hotel_booking}"
COMPOSE_FILE="${COMPOSE_FILE:-/opt/hotel-booking/docker-compose.prod.yml}"
COMPOSE_ENV_FILE="${COMPOSE_ENV_FILE:-/opt/hotel-booking/.env.prod}"

# Retention windows (days). We keep all dumps for 7 days, then only Sunday
# dumps for the next ~12 weeks. Older than 90 days → deleted by this script.
KEEP_DAILY_DAYS=30
KEEP_WEEKLY_WEEKS=12

# Working dir for the temp dump.
TMP_DIR="$(mktemp -d -t pg-backup.XXXXXX)"
trap 'rm -rf "$TMP_DIR"' EXIT

# Date keys.
TS="$(date -u +'%Y%m%dT%H%M%SZ')"
DATE_DAY="$(date -u +'%Y-%m-%d')"
DOW="$(date -u +'%u')"   # 1..7, Mon..Sun
IS_SUNDAY=0
[[ "$DOW" == "7" ]] && IS_SUNDAY=1

# ---- Preflight -------------------------------------------------------------
command -v docker >/dev/null  || fail "docker not on PATH"
command -v b2 >/dev/null      || fail "b2 CLI not on PATH (see header comments)"
command -v gzip >/dev/null    || fail "gzip not on PATH"

log "Starting Postgres backup → b2://$B2_BUCKET/"
log "  container=$POSTGRES_CONTAINER db=$POSTGRES_DB user=$POSTGRES_USER"

# ---- Dump ------------------------------------------------------------------
DUMP_FILE="$TMP_DIR/hotel_booking-$TS.sql.gz"

# Use --format=custom would be faster to restore, but plain SQL + gzip is more
# tool-friendly for ad-hoc inspection. We can revisit.
if ! docker compose --env-file "$COMPOSE_ENV_FILE" -f "$COMPOSE_FILE" \
        exec -T "$POSTGRES_CONTAINER" \
        pg_dump --clean --if-exists --no-owner --no-privileges \
                -U "$POSTGRES_USER" "$POSTGRES_DB" \
    | gzip -9 > "$DUMP_FILE"; then
	fail "pg_dump failed"
fi

DUMP_BYTES="$(stat -c%s "$DUMP_FILE" 2>/dev/null || stat -f%z "$DUMP_FILE")"
if [[ "$DUMP_BYTES" -lt 1024 ]]; then
	fail "dump is suspiciously small ($DUMP_BYTES bytes) — aborting upload"
fi
log "  dump size: $DUMP_BYTES bytes"

# ---- Upload (daily + optionally weekly copy) -------------------------------
DAILY_KEY="daily/hotel_booking-$DATE_DAY.sql.gz"
log "Uploading $DAILY_KEY"
b2 file upload --quiet "$B2_BUCKET" "$DUMP_FILE" "$DAILY_KEY" \
    || fail "b2 upload (daily) failed"

if [[ "$IS_SUNDAY" == "1" ]]; then
	WEEKLY_KEY="weekly/hotel_booking-$DATE_DAY.sql.gz"
	log "Uploading $WEEKLY_KEY"
	b2 file upload --quiet "$B2_BUCKET" "$DUMP_FILE" "$WEEKLY_KEY" \
	    || fail "b2 upload (weekly) failed"
fi

# ---- Retention -------------------------------------------------------------
# Strategy: list files in daily/ and weekly/, delete anything older than the
# cutoff for that bucket. We rely on the date encoded in the key (UTC) so a
# clock skew or restored backup doesn't change "age".
prune_prefix() {
	local prefix="$1" keep_days="$2"
	local cutoff
	cutoff="$(date -u -d "$keep_days days ago" +'%Y-%m-%d' 2>/dev/null \
		|| date -u -v-"${keep_days}d" +'%Y-%m-%d')"
	log "Pruning $prefix older than $cutoff (keep $keep_days d)"
	# `b2 ls` outputs one path per line.
	b2 ls "b2://$B2_BUCKET/$prefix" 2>/dev/null | while read -r key; do
		[[ -z "$key" ]] && continue
		# key looks like: daily/hotel_booking-2026-04-01.sql.gz
		local file_date="${key##*hotel_booking-}"
		file_date="${file_date%%.sql.gz}"
		if [[ "$file_date" < "$cutoff" ]]; then
			log "  deleting $key"
			b2 rm --quiet "b2://$B2_BUCKET/$key" \
			    || log "  WARN: failed to delete $key (continuing)"
		fi
	done
}

prune_prefix "daily/"  "$KEEP_DAILY_DAYS"
prune_prefix "weekly/" "$((KEEP_WEEKLY_WEEKS * 7))"

log "Backup complete."
