# hotel-booking

Direct booking engine + lightweight PMS สำหรับโรงแรมเล็ก

See [plan.md](plan.md) for the full product and architecture plan.

## Monorepo Layout

```
apps/
  api/             Go backend (HTTP API + async workers)
  booking-web/     Next.js — guest-facing landing + booking flow
  admin-web/       Next.js — hotel admin dashboard
infra/             Caddy config, deployment scripts
docker-compose.yml Local dev stack (postgres, redis, minio, imgproxy)
```

## Prerequisites

- Go 1.23+
- Node 20+ and pnpm 9+
- Docker + Docker Compose
- `golang-migrate` CLI for migrations (`brew install golang-migrate`)

## Quick Start

```bash
# 1. Copy env file and adjust
cp .env.example .env

# 2. Start local infrastructure (postgres, redis, minio, imgproxy)
docker compose up -d

# 3. Install JS deps
pnpm install

# 4. Run DB migrations
cd apps/api && make migrate-up

# 5. Start the Go API (in one terminal)
cd apps/api && make dev

# 6. Start booking-web (in another terminal)
cd apps/booking-web && pnpm dev

# 7. Start admin-web (in another terminal)
cd apps/admin-web && pnpm dev
```

Default ports:

- API: `:8080`
- booking-web: `:3000`
- admin-web: `:3001`
- Postgres: `:5432`
- Redis: `:6379`
- MinIO console: `:9001`
- imgproxy: `:8081`

## Scripts

- `pnpm dev` — run both web apps in parallel (root)
- `pnpm build` — build all web apps
- `pnpm lint` — lint all packages
- `make -C apps/api dev` — Go API with hot-reload (air)
- `make -C apps/api migrate-up` / `migrate-down`

## Development Notes

- Self-host stack — see `plan.md` §2 for deployment
- Migrations: numbered SQL files in `apps/api/migrations/`
- Secrets: never commit `.env`. Use `sops` for shared encrypted secrets when needed
