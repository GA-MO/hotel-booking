# 01 — First-time deployment

**When to use:** standing up a new Hetzner server for the first time.
**Time estimate:** 60–90 minutes including DNS propagation.
**Risk level:** Low (no users yet).

> The authoritative source is [`infra/README.md`](../../infra/README.md), which covers Ubuntu hardening, Cloudflare DNS, and the first `docker compose up`. This file is a slim pointer.

## Quick path

```bash
# 1. Provision Hetzner CPX21 (4 vCPU, 8 GB RAM, 80 GB disk, Helsinki, Ubuntu 24.04).
# 2. Follow infra/README.md §1 "One-time server setup" as the `root` user.
# 3. Switch to the deploy user.
ssh deploy@<server-ip>

# 4. Clone + env.
sudo mkdir -p /opt/hotel-booking && sudo chown deploy:deploy /opt/hotel-booking
cd /opt/hotel-booking
git clone git@github.com:GA-MO/hotel-booking.git .
cp infra/.env.prod.example .env.prod
chmod 600 .env.prod
vim .env.prod   # set DOMAIN, *_PASSWORD, JWT_SECRET, IMGPROXY_*, etc.

# 5. Login to GHCR so `docker compose pull` works (PAT with read:packages).
echo "$GHCR_PAT" | docker login ghcr.io -u <github-user> --password-stdin

# 6. First boot — build from source on initial deploy.
docker compose -f docker-compose.prod.yml --env-file .env.prod up -d --build

# 7. Apply migrations.
docker compose -f docker-compose.prod.yml --env-file .env.prod \
  --profile migrate run --rm migrate up

# 8. Verify Caddy obtained certs and the API is healthy.
docker compose -f docker-compose.prod.yml logs caddy | grep -i 'certificate obtained'
curl -sSfI https://api.${DOMAIN}/healthz

# 9. Set up the nightly Postgres → B2 backup cron (deploy user's crontab).
# See infra/backup/backup-postgres.sh header for one-time b2 CLI setup.
crontab -e
# 0 2 * * * B2_BUCKET=hotel-booking-backups \
#   /opt/hotel-booking/infra/backup/backup-postgres.sh \
#   >> /var/log/hotel-backup.log 2>&1
```

## Verify checklist

- [ ] `curl https://api.${DOMAIN}/healthz` → `{"status":"ok"}` with valid TLS
- [ ] `curl https://api.${DOMAIN}/readyz` → 200, both `db` and `redis` "ok"
- [ ] `curl https://book.${DOMAIN}` → 200, HTML loads
- [ ] `curl https://admin.${DOMAIN}` → 200
- [ ] `docker compose -f docker-compose.prod.yml ps` — all services Up + healthy
- [ ] UptimeRobot monitors created (see `infra/README.md` §7)
- [ ] First backup ran successfully (`tail /var/log/hotel-backup.log` after first 2 AM)

## If a step fails

| Symptom | Likely cause | Fix |
|---|---|---|
| `Caddy: timeout obtaining certificate` | Port 80 not reachable through Cloudflare | Verify Cloudflare proxy is enabled (orange cloud) and ufw allows 80/tcp |
| `migrate: pq: SSL connection required` | `sslmode=disable` missing in `DATABASE_URL` | Postgres is on the internal docker network; sslmode=disable is correct |
| `api: connection refused on postgres:5432` | Postgres not yet healthy when API started | Wait, then `docker compose restart api worker` |
| `Resend: 401 unauthorized` | Stale or missing API key | Check `RESEND_API_KEY` in `.env.prod`, redeploy |
