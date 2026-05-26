# Production Deployment Runbook

Hotel Booking Platform — Phase 0 self-hosted on **Hetzner Cloud** behind
**Cloudflare**, orchestrated by **Docker Compose** and reverse-proxied by
**Caddy** with automatic Let's Encrypt SSL.

See [`../plan.md`](../plan.md) §2 (Tech Stack) and §3 (Architecture) for the why.

---

## 0. Topology

```
Cloudflare (proxied DNS, WAF, CDN)
        │  443/80
        ▼
   Hetzner CPX21 — Helsinki
   ├── caddy           (host ports 80/443)
   ├── api             (Go)
   ├── worker          (Go)
   ├── booking-web     (Next.js standalone)
   ├── admin-web       (Next.js standalone)
   ├── postgres        (internal only)
   ├── redis           (internal only)
   ├── minio           (internal only)
   └── imgproxy        (via Caddy at img.${DOMAIN})
```

Public hostnames (point all to the same IP):

| Hostname              | Routes to       |
|-----------------------|-----------------|
| `api.example.com`     | `api:8080`      |
| `book.example.com`    | `booking-web:3000` |
| `admin.example.com`   | `admin-web:3000` |
| `img.example.com`     | `imgproxy:8080` |

---

## 1. One-time server setup (Hetzner CPX21, Ubuntu 24.04)

Provision a **CPX21** (4 vCPU, 8 GB RAM, 80 GB disk) in **Helsinki** with image
**Ubuntu 24.04**. Add your SSH public key during creation.

SSH in as root, then run the following. Replace `<deploy-user-key>` with the
SSH public key you'll use from CI and your laptop.

```bash
# --- 1.1 System updates -----------------------------------------------------
apt-get update && apt-get -y upgrade
apt-get install -y \
    ca-certificates curl gnupg ufw fail2ban unattended-upgrades \
    htop tmux jq vim git pipx

# --- 1.2 Unattended security upgrades --------------------------------------
dpkg-reconfigure -plow unattended-upgrades   # answer "Yes"
# Verify config — should auto-install -security updates:
cat /etc/apt/apt.conf.d/50unattended-upgrades

# --- 1.3 Create a non-root deploy user -------------------------------------
adduser --disabled-password --gecos "" deploy
usermod -aG sudo deploy
mkdir -p /home/deploy/.ssh
echo '<deploy-user-key>' > /home/deploy/.ssh/authorized_keys
chown -R deploy:deploy /home/deploy/.ssh
chmod 700 /home/deploy/.ssh
chmod 600 /home/deploy/.ssh/authorized_keys

# --- 1.4 SSH hardening ------------------------------------------------------
# Edit /etc/ssh/sshd_config — set:
#   PermitRootLogin no
#   PasswordAuthentication no
#   KbdInteractiveAuthentication no
#   X11Forwarding no
sed -i 's/^#*PermitRootLogin.*/PermitRootLogin no/' /etc/ssh/sshd_config
sed -i 's/^#*PasswordAuthentication.*/PasswordAuthentication no/' /etc/ssh/sshd_config
sed -i 's/^#*KbdInteractiveAuthentication.*/KbdInteractiveAuthentication no/' /etc/ssh/sshd_config
systemctl restart ssh

# --- 1.5 Firewall (ufw) -----------------------------------------------------
# Note: We open 22, 80, 443 only. Postgres/Redis/MinIO stay on the internal
# docker network and are NOT exposed to the host. If you ever need to reach
# them from your laptop, use `ssh -L` tunneling — do NOT open the ports here.
ufw default deny incoming
ufw default allow outgoing
ufw allow 22/tcp        # SSH
ufw allow 80/tcp        # HTTP (Caddy + ACME HTTP-01)
ufw allow 443/tcp       # HTTPS
ufw allow 443/udp       # HTTP/3 (QUIC)
ufw --force enable
ufw status verbose

# --- 1.6 Swap (Hetzner CPX21 has no swap by default) ------------------------
fallocate -l 4G /swapfile
chmod 600 /swapfile
mkswap /swapfile
swapon /swapfile
echo '/swapfile none swap sw 0 0' >> /etc/fstab
sysctl -w vm.swappiness=10
echo 'vm.swappiness=10' >> /etc/sysctl.conf

# --- 1.7 fail2ban (default jail.local is fine for sshd) ---------------------
systemctl enable --now fail2ban

# --- 1.8 Install Docker Engine + Compose plugin -----------------------------
install -m 0755 -d /etc/apt/keyrings
curl -fsSL https://download.docker.com/linux/ubuntu/gpg \
    | gpg --dearmor -o /etc/apt/keyrings/docker.gpg
chmod a+r /etc/apt/keyrings/docker.gpg
echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] \
https://download.docker.com/linux/ubuntu $(. /etc/os-release && echo $VERSION_CODENAME) stable" \
    > /etc/apt/sources.list.d/docker.list
apt-get update
apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
usermod -aG docker deploy
# Log out + back in as `deploy` so the group membership takes effect.

# --- 1.9 Set timezone to UTC (recommended) ----------------------------------
timedatectl set-timezone UTC
```

You should now be able to `ssh deploy@<server-ip>` and `docker ps` without sudo.

---

## 2. DNS in Cloudflare

Add **A records** (and AAAA if you assigned IPv6) for the apex and four
subdomains, all pointing at the Hetzner server IP. Recommended setting:
**Proxied (orange cloud)** for everything — Cloudflare handles WAF, DDoS,
and edge cache.

| Type | Name       | Content       | Proxy   | TTL  |
|------|------------|---------------|---------|------|
| A    | `api`      | `<server-ip>` | Proxied | Auto |
| A    | `book`     | `<server-ip>` | Proxied | Auto |
| A    | `admin`    | `<server-ip>` | Proxied | Auto |
| A    | `img`      | `<server-ip>` | Proxied | Auto |
| A    | `@` (apex) | `<server-ip>` | Proxied | Auto |

**SSL/TLS settings in Cloudflare → SSL/TLS:**

- Mode: **Full (strict)** — Caddy will present a real Let's Encrypt cert.
- Edge Certificates → **Always Use HTTPS: On**, **Minimum TLS Version: 1.2**,
  **Automatic HTTPS Rewrites: On**.

> Heads-up on the first cert issuance: when Cloudflare proxy is on, Let's
> Encrypt's HTTP-01 challenge goes through CF. TLS-ALPN-01 does NOT work
> through CF proxy — Caddy will fall back to HTTP-01 automatically, which
> works as long as port 80 reaches the origin (it will, since CF proxies it).

---

## 3. First deploy

```bash
# As `deploy` on the server:
sudo mkdir -p /opt/hotel-booking
sudo chown deploy:deploy /opt/hotel-booking
cd /opt/hotel-booking
git clone https://github.com/<your-org>/hotel-booking.git .

# Copy the env template and fill it in:
cp infra/.env.prod.example .env.prod
chmod 600 .env.prod
vim .env.prod        # set DOMAIN, all *_PASSWORD, JWT_SECRET, IMGPROXY_*, etc.

# First boot — builds images locally (or pulls if IMAGE_TAG is set to a
# specific GHCR tag and you've logged in via `docker login ghcr.io`):
docker compose -f docker-compose.prod.yml --env-file .env.prod pull || true
docker compose -f docker-compose.prod.yml --env-file .env.prod up -d --build

# Wait ~30s, then check Caddy got certificates:
docker compose -f docker-compose.prod.yml logs caddy | grep -i 'certificate obtained'

# Run Postgres migrations (one-shot via the migrate compose profile):
docker compose -f docker-compose.prod.yml --env-file .env.prod \
    --profile migrate run --rm migrate up

# Smoke test:
curl -sSfI https://api.example.com/healthz
curl -sSfI https://book.example.com
curl -sSfI https://admin.example.com
```

---

## 4. Update / redeploy flow

### 4.1 Automated (default)

Push to `main`. The `.github/workflows/deploy.yml` workflow:
1. Builds the three images on GitHub runners.
2. Pushes them to `ghcr.io/<owner>/hotel-booking-*:<sha>` and `:latest`.
3. SSHes to the server and runs `docker compose pull && up -d`.

Skip a deploy by including `[skip deploy]` in the commit message.

### 4.2 Manual

```bash
ssh deploy@<server>
cd /opt/hotel-booking
git pull --ff-only
docker compose -f docker-compose.prod.yml --env-file .env.prod pull
docker compose -f docker-compose.prod.yml --env-file .env.prod up -d
# If a new migration was added:
docker compose -f docker-compose.prod.yml --env-file .env.prod \
    --profile migrate run --rm migrate up
```

---

## 5. Backups

### 5.1 Schedule

See header comments in [`backup/backup-postgres.sh`](backup/backup-postgres.sh)
for full setup. Summary:

```bash
pipx install b2
b2 account authorize "$B2_APPLICATION_KEY_ID" "$B2_APPLICATION_KEY"
b2 bucket create hotel-booking-backups allPrivate

# In `deploy` user's crontab:
0 2 * * * B2_BUCKET=hotel-booking-backups \
          /opt/hotel-booking/infra/backup/backup-postgres.sh \
          >> /var/log/hotel-backup.log 2>&1
```

Retention: 30 daily dumps + 12 weekly (Sunday) dumps, then auto-pruned.

### 5.2 Backup verification (run monthly minimum)

Do this on a **separate** machine — never restore over the live DB.

```bash
# 1. Download the latest daily dump.
b2 file download "b2://hotel-booking-backups/daily/hotel_booking-$(date -u +%F).sql.gz" ./dump.sql.gz

# 2. Spin up a throwaway Postgres.
docker run -d --name pg-restore-test \
  -e POSTGRES_USER=hotel -e POSTGRES_PASSWORD=test \
  -e POSTGRES_DB=hotel_booking \
  -p 55432:5432 postgres:16-alpine

# Wait until ready:
until docker exec pg-restore-test pg_isready -U hotel; do sleep 1; done

# 3. Restore.
gunzip -c dump.sql.gz | docker exec -i pg-restore-test \
    psql -U hotel -d hotel_booking

# 4. Sanity-check a few tables exist & have rows.
docker exec pg-restore-test psql -U hotel -d hotel_booking -c \
    "SELECT relname, n_live_tup FROM pg_stat_user_tables ORDER BY n_live_tup DESC LIMIT 10;"

# 5. Tear down.
docker rm -f pg-restore-test
```

Record the restore time and any anomalies. If restore fails, do not delete the
dump — escalate immediately.

---

## 6. Rollback

Image-tag rollback is the fastest path. The deploy workflow tags each build
with both `:<sha>` and `:latest`, so previous versions stay in GHCR.

```bash
ssh deploy@<server>
cd /opt/hotel-booking

# Find the previous good SHA in GHCR (or in deploy history).
PREV=abc1234

# Pin and restart.
IMAGE_TAG=$PREV docker compose -f docker-compose.prod.yml --env-file .env.prod pull
IMAGE_TAG=$PREV docker compose -f docker-compose.prod.yml --env-file .env.prod up -d
```

If the rollback is required because of a bad migration, you may also need to
restore from the most recent pre-deploy dump (see §5.2 — but onto the live DB
this time, after stopping the api+worker first).

For peace-of-mind, take a manual dump just before any deploy with a schema
change:

```bash
docker compose -f docker-compose.prod.yml exec -T postgres \
    pg_dump -U hotel hotel_booking | gzip > /opt/backups/pre-deploy-$(date -u +%F-%H%M).sql.gz
```

---

## 7. Monitoring (TODO — Phase 0 ops backlog)

- **UptimeRobot** — free tier, 5-min interval. Add HTTPS monitors for:
  - `https://api.example.com/healthz`
  - `https://book.example.com`
  - `https://admin.example.com`
  - SSL cert expiry monitor on each (UptimeRobot has a dedicated check).
- **Sentry** — start with the SaaS free tier (5k events/mo). Self-host is on
  the table for Phase 2 if event volume grows past the free tier.
- **Disk + RAM alerts** — Hetzner Cloud has built-in alerts; enable the
  defaults plus a disk-usage > 80% alert. Also enable Cloudflare HTTP error
  ratio alerts.
- **Log aggregation** — defer until Phase 1. The `json-file` driver with 10 MB
  rotation x 5 files keeps disk usage bounded; `docker compose logs <svc>` is
  enough for now.

---

## 8. Useful one-liners

```bash
# Tail all logs.
docker compose -f docker-compose.prod.yml logs -f --tail=200

# Restart just the api after editing .env.prod.
docker compose -f docker-compose.prod.yml --env-file .env.prod up -d api worker

# Open a psql shell.
docker compose -f docker-compose.prod.yml exec postgres \
    psql -U hotel -d hotel_booking

# Force-renew certs (rarely needed — Caddy does this automatically).
docker compose -f docker-compose.prod.yml exec caddy caddy reload --config /etc/caddy/Caddyfile
```
