# AGENTS.md

> Project context for AI assistants (Claude Code, Cursor, Aider, etc.) working in this repo.
> Read this **first**, then [`plan.md`](plan.md) if you need deeper rationale.

---

## What this is

Hotel-booking SaaS — **direct booking engine + lightweight PMS** for small hotels in Thailand and SE Asia. Each hotel gets its own booking page at `book.<our-domain>/<slug>`; traffic comes from the hotel's own ads and social, **not** from a marketplace search. Revenue is per-hotel subscription (free first year), not commission. Competitors: Cloudbeds, Little Hotelier, Sirvoy. **Not** Agoda/Booking.com.

See [ADR-0001](docs/decisions/0001-subscription-not-commission.md) and [ADR-0002](docs/decisions/0002-direct-booking-not-marketplace.md) for the positioning rationale.

---

## Stack

| Layer | Choice | Notes |
|---|---|---|
| Backend | **Go 1.23**, chi, pgx/v5, slog, envconfig, golang-jwt/v5, argon2 | [ADR-0004](docs/decisions/0004-go-backend-chi-pgx.md) |
| Frontend | **Next.js 15 App Router**, React 19, Tailwind 3 | TS strict; `output: "standalone"` for prod Dockerfile |
| Database | **PostgreSQL 16**, JSONB for flexible content, `FOR UPDATE` for booking race | [ADR-0007](docs/decisions/0007-select-for-update-booking-race.md) |
| Cache | Redis 7 | rate cache, idempotency keys, exchange rates |
| Object storage | MinIO (dev) / Backblaze B2 (prod) | via imgproxy for on-the-fly resize |
| Email | Resend (HTTP, no SDK) | `LogSender` in dev |
| Hosting | **Self-host Hetzner CPX21 Helsinki** + Cloudflare CDN | [ADR-0003](docs/decisions/0003-self-host-hetzner.md) |

**Money is `int64` minor units (satang).** Never use float for money. See [ADR-0006](docs/decisions/0006-money-as-int64-minor-units.md).

---

## Repo layout

```
plan.md                           Full product + architecture plan
AGENTS.md                         This file
docs/decisions/                   ADRs (read the ones relevant to your task)
docs/{architecture,glossary,testing}.md
docs/runbooks/                    Operational playbooks
docs/api/openapi.yaml             API contract (source of truth for FE clients)

apps/api/                         Go API + worker (Go module rooted here)
  cmd/api/main.go                 HTTP server entrypoint
  cmd/worker/main.go              Async worker (booking expiry, notification drain, sub maintenance)
  internal/                       Domain modules — one package per module
    auth/                         signup/login/refresh + AccountInit hook
    hotel/                        hotel CRUD, slug, KYC
    roomtype/                     room types + hotel/room photos
    landing/                      per-locale landing pages (admin + public)
    pricing/                      availability + pricing rules + engine + public quote
    booking/                      reservation state machine + race-safe creation + EventHook
    notification/                 outbox + Resend/LINE/Log senders + retry+backoff
    subscription/                 trial → paid lifecycle (state-only Phase 1)
    platform/{config,db,cache,server,respond}/
    testdb/                       Integration test helpers (TEST_DATABASE_URL)
  migrations/                     SQL migrations 0001-0008
apps/booking-web/                 Next.js 15 — guest-facing landing + booking flow
apps/admin-web/                   Next.js 15 — hotel admin dashboard

docker-compose.yml                Local dev: postgres, redis, minio, imgproxy
docker-compose.prod.yml           Production stack (profile-gated `migrate`)
infra/{caddy,backup,README.md}    Caddy config, Postgres → B2 backup, deploy runbook
scripts/smoke-test.sh             End-to-end happy path
```

---

## Common commands

```bash
# --- one-time setup ---
brew install pnpm golang-migrate
cp .env.example .env

# --- daily dev loop ---
docker compose up -d                      # postgres + redis + minio + imgproxy
make -C apps/api migrate-up               # apply migrations
make -C apps/api dev                      # API with hot reload (air)
pnpm install && pnpm dev                  # both web apps (booking + admin)

# --- testing ---
make -C apps/api test                     # unit tests (integration skip without TEST_DATABASE_URL)
TEST_DATABASE_URL='postgres://hotel:hotel@localhost:5432/hotel_booking?sslmode=disable' \
  make -C apps/api test-integration       # adds -p 1 — see "Gotchas"
./scripts/smoke-test.sh                   # end-to-end against running API

# --- migrations ---
make -C apps/api migrate-create N=add_foo  # new pair of 00XX_*.{up,down}.sql files
make -C apps/api migrate-down              # revert last

# --- production ---
docker compose -f docker-compose.prod.yml --env-file .env.prod up -d
docker compose -f docker-compose.prod.yml --env-file .env.prod \
  --profile migrate run --rm migrate up    # run migrations (Go binary has no migrate subcmd)
```

---

## Code conventions

### Go

- **Module path:** `github.com/GA-MO/hotel-booking/apps/api` — every internal import is fully qualified.
- **Package per domain** under `internal/`. Each has the same six files:
  - `errors.go` — domain sentinel errors (`var ErrFoo = errors.New(...)`)
  - `types.go` — DTOs and entity structs
  - `repository.go` — pgx queries; account-scoped via joins
  - `service.go` — business logic + validation
  - `handler.go` — HTTP handlers + `Routes() chi.Router` (or `AttachTo(r chi.Router)`)
  - `*_test.go` — alongside source files
- **Errors flow:** repository returns sentinel error → service may re-wrap → handler's `writeError` maps to HTTP code via `errors.Is`.
- **Logging:** `slog` (std-lib). Text handler in dev, JSON in prod.
- **No new deps without ADR.** Current deps: chi, pgx, redis-go, envconfig, golang-jwt, google/uuid, golang.org/x/crypto.
- **Time:** store UTC, render in `hotel.timezone`. Dates without time use `time.Time` parsed via `2006-01-02`.
- **Money:** `int64` in minor units (satang). Format at API boundary as `"1500.00"` string.
- **JSON:** `json.Decoder.DisallowUnknownFields()` everywhere user input is decoded — surfaces typos early.
- **Auth context:** middleware sets `auth.Identity` in `ctx`; retrieve via `auth.IdentityFrom(ctx)`. All hotel queries must filter by `identity.AccountID` (404 on cross-tenant).
- **Cross-module wiring:** prefer hooks (function fields with `SetXxxHook(fn)`) over package imports to avoid cycles. Example: `auth.Service` has `SetAccountInit(...)` for subscription, `booking.Service` has `SetEventHook(...)` for notification.

### SQL migrations

- Numbered `00XX_<slug>.{up,down}.sql` pairs.
- Reuse the shared trigger function `set_updated_at()` defined in `0001_initial_schema.up.sql`.
- All FKs use `ON DELETE CASCADE` for child→parent relationships within an account; `ON DELETE RESTRICT` only for cross-aggregate refs (bookings → hotels).
- Money columns: `BIGINT` named `*_cents`.
- Money rates: `NUMERIC(10,2)` if user-facing (`base_rate`, `rate_override`).
- Timestamps: `TIMESTAMPTZ NOT NULL DEFAULT NOW()`.
- Soft-delete columns: `deleted_at TIMESTAMPTZ` only where business needs it (accounts, users, hotels, room_types). Hard delete elsewhere.

### TypeScript / Next.js

- Strict TS, App Router only, no Pages Router.
- Tailwind 3 (NOT v4 — see [ADR memo not yet written]).
- `output: "standalone"` in both `next.config.mjs` (for Docker).
- Public env: `NEXT_PUBLIC_API_URL`, `NEXT_PUBLIC_BOOKING_BASE_URL`.

---

## Gotchas (read before you debug)

1. **`go test -p 1` for integration tests.** Multiple packages running `TRUNCATE` in parallel deadlock against each other. Within a package, tests run serially (no `t.Parallel()` in integration tests). `make test-integration` already adds `-p 1`.

2. **Booking race condition.** `booking.Repository.CreatePending` opens a tx, then `SELECT … FOR UPDATE` on the `room_types` row. Don't refactor away the lock. The 10-goroutine integration test (`TestRepo_CreatePending_RaceNoOversell`) catches regressions. See [ADR-0007](docs/decisions/0007-select-for-update-booking-race.md).

3. **chi.Mount("/") conflict.** Mounting two sub-routers at `/` under the same parent panics at startup. `roomtype` and `pricing` both have multiple top-level paths under `/v1/hotels/{hotel_id}` — they expose `AttachTo(chi.Router)` for this reason. New modules with multiple paths should follow the same pattern.

4. **Money math.** `room_subtotal_cents`, `total_cents` etc. are `int64`. Pricing engine works in satang throughout; only the JSON boundary converts to `"1500.00"` strings. Don't introduce `float64` for amounts.

5. **Resend API is hand-rolled.** No SDK dep. See `internal/notification/sender.go` `ResendSender`. If Resend changes their API, fix here.

6. **Argon2 is slow on purpose.** ~100ms per hash. Tests that hash should do so once or twice per file, not per case.

7. **Auto-mode blocks pushing to `main` directly.** Use a feature branch + PR. The maintainer pushes to `main` themselves.

8. **Hotel must be `status='live'`** for `/v1/public/hotels/{slug}/bookings` to accept guest bookings; admin/walk-in path (`/v1/hotels/{id}/bookings`) accepts test-mode hotels.

9. **Date arithmetic.** `check_in_date`, `check_out_date` are stored as `DATE` and parsed with the layout `2006-01-02`. Number of nights = `check_out - check_in` (exclusive). Hotel timezone matters for "today" semantics — don't use server time.

10. **Notifications outbox.** Booking lifecycle calls `notification.Enqueue*` through a hook (`booking.Service.SetEventHook`). The worker drains via `FOR UPDATE SKIP LOCKED` — see [ADR-0008](docs/decisions/0008-outbox-pattern-notifications.md). Don't call senders synchronously from request handlers.

---

## Where to add X

| You want to… | Look at | Then |
|---|---|---|
| Add a new domain module | `internal/hotel/` as a template | mirror the 6-file layout; wire into `platform/server/server.go` |
| Add a new endpoint to an existing module | that module's `handler.go` | follow `requireRole(…)` / `parseUUIDParam` / `decodeJSON` / `writeError` pattern |
| Add a new column | `migrations/00NN_*.up.sql` + `.down.sql` | also update `selectColumns` in repository |
| Send an email | `internal/notification/templates.go` for a new template | enqueue via `service.Enqueue(...)` from a hook |
| Add a config field | `internal/config/config.go` + `.env.example` | optionally surface in `AGENTS.md` if it's a runtime gotcha |
| Add a background job | `cmd/worker/main.go` `runJobs` | keep it idempotent and bounded; log a tick summary only when something happened |
| Add a CI check | `.github/workflows/ci.yml` | match the existing `api` + `web` job split |

---

## What's done vs in-progress

**Done (Phase 0 + Phase 1 backend):**

- auth · hotel · roomtype · landing · pricing · booking · notification · subscription — all 8 domain modules with unit tests + integration tests against real Postgres
- 14 tables across migrations 0001–0008
- Worker: booking expiry + notification drain + subscription maintenance
- Cross-module hooks closed: signup → ensure subscription; booking events → enqueue notification
- Production scaffold: docker-compose.prod.yml, Caddyfile, B2 backup, GitHub Actions CI/deploy

**In progress / Phase 2:**

- Frontend: booking-web ISR rendering + admin-web onboarding wizard (currently stubs only)
- Image upload pipeline (MinIO presigned URLs)
- Stripe/Omise SDK in subscription module (state-only today)
- LINE Messaging delivery (stub today)
- Channel manager (SiteMinder/Hotellink) — Phase 3
- e-Tax invoice (Leceipt) — Phase 2

See [`plan.md` §9 roadmap](plan.md) for full sequencing.

---

## When in doubt

1. Search the failing area's `*_test.go` for the existing pattern.
2. Read the relevant ADR in `docs/decisions/`.
3. If a decision needs to be made, write a new ADR (`docs/decisions/00NN-….md`, status="Proposed") and link it from the PR.
4. Ask the maintainer rather than introducing a new dependency or pattern.
