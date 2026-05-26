# AGENTS.md

> Project context for AI assistants (Claude Code, Cursor, Aider, etc.).
> Read this first. For deeper rationale see [`plan.md`](plan.md); for non-obvious decisions see [`docs/decisions/`](docs/decisions/).

## What this is

Hotel-booking SaaS — direct booking engine + lightweight PMS for small hotels in Thailand and SE Asia. Each hotel gets its own page at `book.<domain>/<slug>`; traffic comes from the hotel's own ads, **not** from a marketplace search. Per-hotel subscription, **not** commission. Competitors: Cloudbeds, Little Hotelier, Sirvoy — **not** Agoda/Booking.com. See `plan.md §1, §11` for positioning.

## Stack

- **Backend** Go 1.23 — chi, pgx/v5, slog, envconfig, golang-jwt/v5, argon2id
- **Frontend** Next.js 15 App Router, React 19, Tailwind 3 (`output: "standalone"`)
- **Database** PostgreSQL 16 (JSONB + `FOR UPDATE` for booking race — [ADR-0007](docs/decisions/0007-select-for-update-booking-race.md))
- **Cache** Redis 7
- **Object storage** MinIO (dev) / Backblaze B2 (prod) via imgproxy
- **Email** Resend (HTTP, no SDK; `LogSender` in dev)
- **Hosting** Self-host Hetzner CPX21 Helsinki + Cloudflare (see `plan.md §2.1`)

**Money is `int64` minor units (satang).** Never `float`. See [ADR-0006](docs/decisions/0006-money-as-int64-minor-units.md).

## Repo layout

```
plan.md                  Strategic plan (slow-changing)
AGENTS.md                This file
CHANGELOG.md             Release notes (update on tag)
docs/
  decisions/             ADRs — only for non-obvious decisions
  runbooks/              Playbooks for ops we've actually done
  glossary.md            TH/EN domain + tech terms
  api/openapi.yaml       API contract (source of truth for FE codegen)

apps/api/                Go API + worker (module rooted here)
  cmd/{api,worker}/      Two entrypoints, same internal/ packages
  internal/
    {auth, hotel, roomtype, landing, pricing, booking,
     notification, subscription}/   domain modules
    platform/{config,db,cache,server,respond}/
    testdb/                          integration-test helpers
  migrations/                        SQL 0001-0008

apps/{booking-web,admin-web}/        Next.js 15 (scaffold-only Phase 1)

docker-compose.yml                   Dev: postgres, redis, minio, imgproxy
docker-compose.prod.yml              Prod stack (profile-gated `migrate`)
infra/                               Caddyfile, B2 backup, deploy runbook
scripts/smoke-test.sh                End-to-end happy path
```

## Common commands

```bash
# one-time
brew install pnpm golang-migrate
cp .env.example .env

# daily dev
docker compose up -d                       # postgres + redis + minio + imgproxy
make -C apps/api migrate-up
make -C apps/api dev                       # API hot-reload (air)
pnpm install && pnpm dev                   # both web apps

# tests
make -C apps/api test                      # unit (integration skips without TEST_DATABASE_URL)
make -C apps/api test-integration          # adds -p 1 — see "Gotchas"
./scripts/smoke-test.sh                    # e2e against running API

# migrations
make -C apps/api migrate-create N=add_foo
make -C apps/api migrate-down

# production migrations
docker compose -f docker-compose.prod.yml --env-file .env.prod \
  --profile migrate run --rm migrate up    # Go binary has no `migrate` subcommand
```

## Code conventions

### Go

- **Module path:** `github.com/GA-MO/hotel-booking/apps/api`. Imports are fully qualified.
- **Package per domain** under `internal/`. Six-file shape: `errors.go`, `types.go`, `repository.go`, `service.go`, `handler.go`, `*_test.go`. Add `doc.go` for package-level invariants.
- **Error flow:** repository → sentinel error → service may wrap → handler's `writeError` maps to HTTP via `errors.Is`.
- **Logging:** `slog` (text in dev, JSON in prod).
- **No new deps without a PR-level discussion.** Current: chi, pgx, redis-go, envconfig, golang-jwt/v5, google/uuid, golang.org/x/crypto.
- **Time:** store UTC, render in `hotel.timezone`. Date-only fields parsed with `2006-01-02`.
- **Money:** `int64` in minor units (satang); format `"1500.00"` at API boundary.
- **JSON:** `json.Decoder.DisallowUnknownFields()` on every decode of user input.
- **Auth context:** middleware sets `auth.Identity` in ctx → `auth.IdentityFrom(ctx)`. Every query must filter by `identity.AccountID`. Cross-tenant returns **404** (no existence disclosure), never 403.
- **Cross-module wiring:** prefer hooks (`SetXxxHook(fn)`) over package imports — e.g. `auth.Service.SetAccountInit`, `booking.Service.SetEventHook`. Avoids cycles.

### SQL migrations

- Numbered `00XX_<slug>.{up,down}.sql` pairs.
- Reuse `set_updated_at()` from `0001_initial_schema.up.sql`.
- Child→parent FKs in an account use `ON DELETE CASCADE`; cross-aggregate FKs use `ON DELETE RESTRICT`.
- Money columns: `BIGINT` named `*_cents`. User-facing rates: `NUMERIC(10,2)`.
- Timestamps: `TIMESTAMPTZ NOT NULL DEFAULT NOW()`.
- Soft-delete only where business needs it (accounts, users, hotels, room_types).

### TypeScript / Next.js

- Strict TS, App Router only.
- Tailwind 3 (not v4 — different config model).
- `output: "standalone"` in `next.config.mjs` for Docker.
- Public env: `NEXT_PUBLIC_API_URL`, `NEXT_PUBLIC_BOOKING_BASE_URL`.

## Gotchas

1. **`go test -p 1` for integration tests.** Parallel `TRUNCATE` across packages deadlocks. `make test-integration` already passes `-p 1`. Within a package, no `t.Parallel()` in integration tests.
2. **Booking race condition.** `booking.Repository.CreatePending` opens a tx, then `SELECT … FOR UPDATE` on the `room_types` row. **Don't refactor the lock away** — the 10-goroutine integration test (`TestRepo_CreatePending_RaceNoOversell`) catches regressions. See [ADR-0007](docs/decisions/0007-select-for-update-booking-race.md).
3. **`chi.Mount("/")` conflict.** Two sub-routers at `/` under the same parent panics. `roomtype` and `pricing` both have multiple top-level paths under `/v1/hotels/{hotel_id}` and expose `AttachTo(chi.Router)` for this reason. Follow that pattern.
4. **Money math.** `*_cents` columns are `int64`. Pricing engine works in satang throughout; the JSON boundary formats to `"1500.00"` strings.
5. **Resend API is hand-rolled.** No SDK dep. See `internal/notification/sender.go`. If Resend changes their wire format, fix here.
6. **Argon2 is slow on purpose.** ~100ms per hash. Tests that hash should do so once or twice per file.
7. **Auto-mode blocks pushes to `main`.** Use a feature branch + PR.
8. **Public bookings require `hotel.status='live'`.** Admin walk-in path accepts test-mode hotels.
9. **Date arithmetic.** `check_in_date`, `check_out_date` are `DATE`; check-out is exclusive (`nights = check_out − check_in`). Use hotel timezone, not server time.
10. **Notifications go through the outbox.** Booking lifecycle fires `notification.Enqueue*` via a hook ([ADR-0008](docs/decisions/0008-outbox-pattern-notifications.md)). Don't call senders synchronously from request handlers.
11. **`chi` shadows sibling `Mount("/x")` + `Route("/x/{id}")`.** Putting top-level CRUD via `Mount` next to a `Route` for per-id sub-resources at the same parent makes chi pick the sub-tree, leaving the bare `/{id}` handlers unreachable. The fix on this repo: single `Route("/hotels", ...)` group with `AttachCollection` + `AttachByID` (see `internal/hotel/handler.go`).
12. **DATE columns are `datetypes.Date`, not `time.Time`.** `time.Time` serializes as full RFC3339 and breaks the FE's `YYYY-MM-DD` parser. Use `internal/platform/datetypes.Date` for any column declared `DATE` in SQL (today: `bookings.check_in_date` / `check_out_date`). It also implements `sql.Scanner` so pgx scans transparently.
13. **Always prefix columns in JOIN-able SELECT lists.** A bare `id` in a constant like `bookingColumns` becomes `42702 ambiguous` the day someone adds a `JOIN hotels` to one of the queries that uses it. Keep the constant prefixed (`b.id, b.reference, ...`) and add `AS b` to every INSERT/UPDATE that returns through it.
14. **Smoke + integration in CI catch what `go test` cannot.** `scripts/smoke-test.sh` and the `api-integration` workflow job hit real Postgres + real HTTP. If a change touches SQL, route mounting, JSON wire shapes, or the booking state machine, run the smoke locally before pushing — unit tests alone have missed this category of bug repeatedly.

## Where to add X

| You want to… | Look at | Then |
|---|---|---|
| Add a new domain module | `internal/hotel/` as template | mirror 6-file layout; wire into `platform/server/server.go` |
| Add a new endpoint to existing module | that module's `handler.go` | follow `requireRole / parseUUIDParam / decodeJSON / writeError` |
| Add a new column | `migrations/00NN_*.up.sql` + `.down.sql` | also update `selectColumns` in repository |
| Send an email | `notification/templates.go` for new template | enqueue via `service.Enqueue(...)` from a hook |
| Add a config field | `internal/config/config.go` + `.env.example` | surface here if it's a runtime gotcha |
| Add a background job | `cmd/worker/main.go` `runJobs` | keep idempotent + bounded; log summary only when something happened |
| Add a route | the handler + `server.go` mount | also add it to `server_test.go` expected-routes |
| Add an OpenAPI path | `docs/api/openapi.yaml` (hand-written today) | CI lints — keep it in sync with handlers |

## What's done vs in-progress

**Done — Phase 0 + Phase 1 backend:**
- 8 domain modules (auth, hotel, roomtype, landing, pricing, booking, notification, subscription) with unit + integration tests against real Postgres
- 14 tables across migrations 0001–0008
- Worker: booking expiry + notification drain + subscription maintenance
- Cross-module hooks: signup → subscription init; booking events → notification enqueue
- Production: docker-compose.prod.yml, Caddyfile, B2 backup, GitHub Actions CI/deploy

**In progress — Phase 2:**
- Frontend (booking-web ISR landing, admin-web onboarding wizard)
- Image upload pipeline (MinIO presigned URLs)
- Stripe/Omise SDK in subscription (state-only today)
- LINE Messaging delivery (stub today)
- e-Tax invoice (Leceipt) — Phase 2
- Channel manager — Phase 3

See [`plan.md §9`](plan.md) for the full roadmap.

## When in doubt

1. Search the failing area's `*_test.go` for the existing pattern.
2. Read the relevant `doc.go` and any linked ADR.
3. If you're tempted to add a dependency, contradict an ADR, or introduce a new pattern — open a PR with rationale rather than doing it silently. For new decisions, propose an ADR (`docs/decisions/00NN-….md`, status="Proposed") and link it.
