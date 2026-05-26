# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html)
once it ships its first tagged release.

Until 1.0, every minor version may include breaking changes. Migration notes
will be called out under each release.

---

## [Unreleased]

### Added — Phase 2 frontend (booking-web, guest)

- `/[slug]` landing page — SSR + ISR (`revalidate=60`), branded sections (hero, gallery, rooms, amenities, location, reviews), SEO metadata, tracking-pixel injection (FB, GA, GTM).
- `/[slug]/book` checkout — date picker + room-type select + live quote (`POST /v1/public/quote/{slug}`) + guest form + `POST /v1/public/hotels/{slug}/bookings`.
- `/[slug]/booking/{reference}` confirmation — booking summary, PromptPay QR (`qrcode` lib, placeholder payload), cancel-by-guest, email lookup form.
- `i18n` TH (default) + EN dictionaries, locale switch via `?lang=` or `Accept-Language`.
- Shared utilities: `api.ts` (typed fetch with error-envelope handling), `money.ts` (satang → display), `dates.ts` (timezone-aware, exclusive checkout).

### Added — Phase 2 frontend (admin-web, hotel staff)

- Auth: `/login`, `/signup`, `/logout` with refresh-on-401 (serialized in-flight promise) + `localStorage` token storage (httpOnly-cookie migration on P3 backlog).
- 3-step onboarding wizard (`/onboarding`): hotel basics → first room type → first landing page.
- Dashboard: today's check-ins, occupancy snapshot, recent bookings, subscription banner.
- Bookings list + detail with all 5 status actions (confirm / check-in / check-out / cancel / no-show).
- Calendar month grid with click-to-walk-in modal.
- Hotel settings, room-types CRUD, landing-page editor (per-locale, branding + sections + SEO + tracking + publish/unpublish), pricing-rules CRUD, subscription view.
- Multi-hotel switcher, TH/EN locale switch.

### Added — image upload pipeline

- `apps/api/internal/upload/` new Go module — six-file shape + tests.
- `POST /v1/uploads/presign` — S3 SigV4 presigned PUT URL (hand-rolled, no `aws-sdk-go-v2`; 20 unit tests, 78% coverage; AWS-published key-derivation test vector validates the HMAC chain).
- `POST /v1/uploads/imgproxy-url` — HMAC-SHA256 signed delivery URL with width/height/format/resize options.
- Storage config (`STORAGE_*` env vars), imgproxy config (`IMGPROXY_KEY` / `IMGPROXY_SALT`).
- `docker-compose.yml` already had a `minio-init` one-shot service that creates the bucket with public-read on first boot — left unchanged.
- admin-web wired: `PhotoUploader` component, room-type photo UI now uploads files directly via presign → PUT → `POST /v1/hotels/{id}/room-types/{id}/photos` with the returned `storage_key`.

### Changed

- **BREAKING:** `GET /v1/public/landing/{slug}/{locale}` now returns `PublicLandingResponse` (= `LandingPage` + `{ hotel: { name, slug, timezone, currency } }`) so the guest UI can format dates in hotel-local time without a second round-trip.
- booking-web confirmation page fetches landing best-effort to populate the timezone, falling back to UTC if the hotel is unlisted or no published page exists for that locale.

### Added — documentation

- `AGENTS.md` at repo root — AI-assistant onboarding (also useful for humans).
- `docs/decisions/` — 4 ADRs for non-obvious / counter-intuitive decisions only (money int64, FOR UPDATE booking race, outbox-pattern notifications, structured-not-page-builder landing). Other decisions live in `plan.md §12`.
- `docs/glossary.md` — bilingual TH/EN glossary of hospitality, pricing, technical terms.
- `docs/runbooks/` — operational playbooks: deployment (verified) + rollback (unverified).
- `doc.go` in every Go domain package — package-level invariants + ADR cross-refs.
- `docs/api/openapi.yaml` — hand-written OpenAPI 3.1 spec, now ~44 paths (added `/v1/uploads/*`).

### Changed — documentation

- Pared the initial 10 ADRs down to 4 (the rest restated plan.md content); see `docs/decisions/README.md` for the criteria.
- Trimmed `AGENTS.md` from 7KB → ~5KB (cut sections that duplicated `plan.md`).
- Removed speculative runbooks (restore-from-backup, overbooking, JWT rotation) — to be written when the operation is performed for the first time.
- Removed `docs/architecture.md` and `docs/testing.md` — folded into `AGENTS.md`.

### Known gaps (carry-over into next round)

- PromptPay QR payload is a placeholder string — needs the hotel's PromptPay ID exposed on a public endpoint to render a real EMVCo QR.
- Booking confirmation `GET /v1/public/bookings/{reference}` does not yet expose `timezone` — landing fallback covers it for now.
- Hotel logo (`landing.branding.logo_url`) and KYC documents in admin-web are still URL text inputs; wire them to `Uploads.upload()` in a follow-up.
- admin-web tokens still in `localStorage` (XSS-readable); migration to httpOnly cookies via a Next.js route-handler proxy on the P3 backlog.
- Bulk availability-override editor stubbed; backend client ready.
- `booking_events` audit trail reconstructed from booking timestamp columns; a dedicated `GET /v1/hotels/{id}/bookings/{id}/events` endpoint would unlock richer history.
- B2 production bucket CORS rules need a runbook when the prod bucket is provisioned.

---

## [0.1.0-phase1-backend] — 2026-05-26

The Phase 1 backend is feature-complete. All 8 domain modules, 14 Postgres tables across 8 migrations, ~58 endpoints, and the production deployment scaffold are in place. End-to-end smoke test passes against real Postgres.

### Added — domain modules

- **`auth`** — signup/login/refresh/logout/me. argon2id passwords; HS256 short-lived access JWT + opaque rotating refresh tokens with token-reuse detection. `AccountInitFn` hook for post-signup side effects.
- **`hotel`** — CRUD, slug uniqueness, KYC, multi-photo. Cross-tenant access returns 404.
- **`roomtype`** — room types with shared inventory; hotel + room-type photo attachments with at-most-one-cover enforcement.
- **`landing`** — per-(hotel, locale) landing pages: branding, ordered section list, SEO, tracking pixel IDs. Whitelisted section type catalog. Draft/published lifecycle. Public anonymous read endpoint gated by `hotel.status='live' AND landing.status='published'`.
- **`pricing`** — `availability_overrides` + `pricing_rules` (season / day-of-week / length-of-stay / advance-purchase). Pure-int satang pricing engine. Public `/quote/{slug}` preview endpoint (no inventory subtraction).
- **`booking`** — reservation lifecycle with `SELECT ... FOR UPDATE` race-safe creation. 9-char `HB-XXXXXX` references using a 28-char vowel-free alphabet. State machine: pending_payment → confirmed → checked_in → checked_out, plus cancel / expire / no_show branches. `SetEventHook` for external effects.
- **`notification`** — outbox pattern. ResendSender + LogSender + LineSender (stub). `FOR UPDATE SKIP LOCKED` claim semantics. Bilingual TH/EN templates. Retry policy: 5 attempts, 2/4/8/16/32-minute backoff.
- **`subscription`** — full state machine (pending_kyc → trialing → trial_ending → trial_lapsed → suspended → cancelled → terminated). `EnsureForAccount` idempotent. Worker maintenance scan auto-advances time-based transitions.

### Added — infrastructure & ops

- `docker-compose.yml` (dev: postgres, redis, minio, imgproxy).
- `docker-compose.prod.yml` (prod: + caddy + api + worker + web apps, with profile-gated `migrate` one-shot).
- `infra/caddy/Caddyfile` for 4 subdomains (api, book, admin, img) with HSTS + gzip+zstd + Cloudflare trust list.
- `infra/backup/backup-postgres.sh` — nightly pg_dump → Backblaze B2, 30 daily + 12 weekly retention with auto-prune.
- `infra/README.md` — Hetzner CPX21 first-time setup, DNS, deploy, redeploy, backup verify, rollback.
- `.github/workflows/ci.yml` — Go build + test + pnpm build + typecheck.
- `.github/workflows/deploy.yml` — matrix builds → GHCR → SSH deploy with `[skip deploy]` opt-out.
- `apps/api/Dockerfile` and per-app web Dockerfiles (Next.js standalone output).
- `scripts/smoke-test.sh` — end-to-end signup → booking → confirm → check-in/out + overbook 409.

### Added — tests

- Unit tests in every domain module (table-driven, race-clean, no testify).
- Integration tests in `auth`, `hotel`, `booking`, `notification`, `subscription` using `internal/testdb` helpers and `TEST_DATABASE_URL`. The flagship `booking.TestRepo_CreatePending_RaceNoOversell` test fires 10 concurrent goroutines at an inventory of 3 and asserts exactly 3 succeed.
- `platform/server/server_test.go` walks the chi router asserting every expected route is registered — catches `chi.Mount("/")` conflicts at test time.

### Cross-module wiring

- `auth.Service.SetAccountInit(fn)` — server wires `subscription.EnsureForAccount` so a sub row exists immediately on signup.
- `booking.Service.SetEventHook(fn)` — server wires it to look up hotel name + locale and call `notification.EnqueueBookingCreated/Confirmed/Cancelled`.
- Worker (`cmd/worker`) every 60s: `booking.ExpirePending` + `notification.DispatchPending` + `subscription.RunMaintenance`.

### Fixed (during integration)

- `notification.Repository.ClaimBatch` — CTE column rename to disambiguate the `RETURNING` clause (Postgres 42702 on the original `RETURNING id`).
- `chi.Mount("/")` panic when `roomtype` and `pricing` both wanted the same prefix — added `AttachTo(chi.Router)` to both modules.

### Database migrations

| # | Adds |
|---|---|
| 0001 | accounts, users, hotels, room_types + `set_updated_at()` trigger function |
| 0002 | sessions (refresh tokens) |
| 0003 | hotel_photos, room_type_photos (with partial UNIQUE index on `is_cover = TRUE`) |
| 0004 | landing_pages |
| 0005 | availability_overrides, pricing_rules |
| 0006 | bookings (with `nights` generated column), booking_events |
| 0007 | notifications (outbox) |
| 0008 | subscriptions, subscription_events |

### Decisions recorded

ADRs 0001 – 0010 (see `docs/decisions/`):

- 0001 Subscription not commission
- 0002 Direct booking not marketplace
- 0003 Self-host on Hetzner
- 0004 Go backend (chi + pgx)
- 0005 argon2id passwords
- 0006 Money as int64 minor units
- 0007 SELECT FOR UPDATE for booking race
- 0008 Outbox pattern for notifications
- 0009 Structured template not page builder
- 0010 Pricing tiers deferred pending customer research

### Known limitations

- Frontend is scaffold-only (Phase 2): the `[slug]` route on booking-web returns 404; admin-web has only a placeholder home.
- Stripe/Omise SDKs are not yet integrated in `subscription` — state-only, no real charges.
- LINE Messaging API delivery is a stub; only email (Resend) actually sends.
- Image upload pipeline (MinIO presigned URLs) deferred to Phase 2.
- e-Tax invoice generation (Leceipt) deferred to Phase 2.
- Channel manager integration (SiteMinder / Hotellink / MyAllocator) deferred to Phase 3.
- Integration tests are not yet wired into CI (need a Postgres service container).

---

## How to add an entry

When you ship a notable change, prepend a bullet under `[Unreleased]` in the most appropriate section: **Added**, **Changed**, **Deprecated**, **Removed**, **Fixed**, **Security**. When cutting a release, rename `[Unreleased]` to `[X.Y.Z] — YYYY-MM-DD` and start a fresh `[Unreleased]` block above it.

For breaking changes, prefix the entry with `**BREAKING:**` and include a sub-bullet with the migration path.
