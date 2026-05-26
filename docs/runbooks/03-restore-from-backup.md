# 03 — Restore Postgres from Backblaze B2

**When to use:**
- Routine restore verification (quarterly minimum — never let a backup go unverified).
- Recovery from data corruption, accidental TRUNCATE, or a destructive migration that bypassed safer alternatives.
**Time estimate:** 10–20 minutes including verification.
**Risk level:** High when restoring over a live DB. Low when restoring to a throwaway container for verification.

## Daily backup recap

- Cron at 02:00 UTC runs `infra/backup/backup-postgres.sh` as the `deploy` user.
- Writes `daily/hotel_booking-YYYY-MM-DD.sql.gz` to the configured B2 bucket.
- Sunday runs also write `weekly/hotel_booking-YYYY-MM-DD.sql.gz`.
- Retention: 30 days of dailies + 12 weeks of weeklies. Older files are auto-pruned by the same script.

## A. Verification restore (recommended path; quarterly)

> Do this on your laptop or a sandbox VM — **never** on the production server's main Postgres.

```bash
# 1. Pull the latest daily dump from B2.
b2 file download "b2://hotel-booking-backups/daily/hotel_booking-$(date -u +%F).sql.gz" ./dump.sql.gz

# 2. Spin up a throwaway Postgres on a non-standard port.
docker run -d --rm --name pg-restore-test \
  -e POSTGRES_USER=hotel -e POSTGRES_PASSWORD=test \
  -e POSTGRES_DB=hotel_booking \
  -p 55432:5432 postgres:16-alpine

until docker exec pg-restore-test pg_isready -U hotel; do sleep 1; done

# 3. Restore.
gunzip -c dump.sql.gz | docker exec -i pg-restore-test psql -U hotel -d hotel_booking

# 4. Sanity-check row counts.
docker exec pg-restore-test psql -U hotel -d hotel_booking -c "
SELECT relname AS table, n_live_tup AS rows
FROM pg_stat_user_tables
ORDER BY n_live_tup DESC LIMIT 15;"

# 5. Confirm a recent booking exists.
docker exec pg-restore-test psql -U hotel -d hotel_booking -c "
SELECT reference, status, created_at FROM bookings
ORDER BY created_at DESC LIMIT 5;"

# 6. Tear down.
docker rm -f pg-restore-test
```

Record the restore time and any anomalies in `docs/runbooks/backup-verifications.log` (create if missing).

If anything fails — **do not delete the dump**. Page the maintainer immediately; a non-restorable backup is an outage waiting to happen.

## B. Production restore (last resort)

> Read this entire section before starting.

### Pre-requisites

- [ ] You've verified that the loss is real and not, say, a single misbehaving row that a targeted UPDATE could fix.
- [ ] You've identified the exact dump file (by timestamp) that predates the loss.
- [ ] You've notified the team in `#incidents` or equivalent.
- [ ] The deploy has been frozen — no new commits to main.

### Steps

```bash
ssh deploy@<server>
cd /opt/hotel-booking

# 1. Stop services that write to Postgres.
docker compose -f docker-compose.prod.yml stop api worker

# 2. Take a fresh dump of the CURRENT (broken) state as a safety net.
docker compose -f docker-compose.prod.yml exec -T postgres \
  pg_dump -U hotel hotel_booking | gzip > \
  /opt/backups/pre-restore-$(date -u +%F-%H%M).sql.gz

# 3. Download the target dump.
b2 file download "b2://hotel-booking-backups/daily/hotel_booking-YYYY-MM-DD.sql.gz" ./target.sql.gz

# 4. Drop + recreate the database.
docker compose -f docker-compose.prod.yml exec -T postgres \
  psql -U hotel -d postgres -c "DROP DATABASE hotel_booking;"
docker compose -f docker-compose.prod.yml exec -T postgres \
  psql -U hotel -d postgres -c "CREATE DATABASE hotel_booking OWNER hotel;"

# 5. Restore.
gunzip -c target.sql.gz | docker compose -f docker-compose.prod.yml exec -T postgres \
  psql -U hotel -d hotel_booking

# 6. Apply any migrations that landed AFTER the dump (if you know it's safe to do so).
docker compose -f docker-compose.prod.yml --env-file .env.prod \
  --profile migrate run --rm migrate up

# 7. Bring services back.
docker compose -f docker-compose.prod.yml --env-file .env.prod up -d api worker

# 8. Smoke test.
./scripts/smoke-test.sh
```

### After

- [ ] Note the gap: bookings/payments made between the dump time and the restore are lost. Identify affected guests via email and call them directly.
- [ ] Write an incident note (`docs/runbooks/incidents/YYYY-MM-DD-….md`) — root cause, timeline, customer impact, prevention.
- [ ] If the loss was caused by a bug, file a regression test in the appropriate `*_test.go`.
