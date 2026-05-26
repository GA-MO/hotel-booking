# ADR-0004 — Go backend with chi + pgx + slog + envconfig

- **Status:** Accepted
- **Date:** 2026-05-26
- **Deciders:** team
- **Related:** plan.md §2 (Tech Stack), §3 (Architecture)

## Context

The backend has to do a few things well: serve a JSON HTTP API, talk to
Postgres a lot, run background workers (notification drain, booking
expiry, subscription maintenance), and stay cheap to run on a 4 vCPU /
8 GB Hetzner box (ADR-0003). It is not CPU-bound and it is not heavily
async — mostly request/response and short-lived workers polling the DB.

The realistic language choices were Node.js (NestJS / Fastify),
Python (FastAPI), or Go. We also had to choose libraries within whichever
language we picked. Inside Go, the common axes are:

- **Router:** chi, echo, gin, gorilla/mux, std-lib net/http (since 1.22).
- **DB driver:** `database/sql` + `lib/pq`, `database/sql` + `pgx/v5`, pgx native.
- **Logging:** logrus, zap, std-lib `slog` (added in 1.21).
- **Config:** viper, envconfig, hand-rolled.

## Decision

Backend is Go. Within Go: **chi** for routing, **pgx/v5 native** (not via
`database/sql`) for Postgres, **`log/slog`** for structured logging, and
**envconfig** for env-driven config. Worker is the same binary built from
`cmd/worker`, sharing all internal packages.

## Consequences

### Positive
- **Go:** single static binary, ~10–30 MB RSS per process, no per-request goroutine cost worth measuring. Compile-time errors catch most type bugs before integration tests.
- **chi:** closest router to std-lib `net/http` idioms — the handler signature is `http.HandlerFunc`, middleware is plain `func(http.Handler) http.Handler`. Small surface, no DSL to learn.
- **pgx native** (not `database/sql`): native Postgres types (uuid, jsonb, arrays) without scan acrobatics; copy protocol; faster than `lib/pq`; `pgxpool` is its own connection pool.
- **slog:** stdlib since 1.21, structured by default, no third-party dep.
- **envconfig:** simpler than viper for our needs — no config files, no hot reload, just `process.Process("HB", &cfg)`.

### Negative / trade-offs
- Smaller ecosystem for hospitality / payment libs vs Node or Python — for example, Omise's Node SDK is canonical and we end up writing the HTTP client ourselves (see `internal/notification/ResendSender`).
- Generic-friendly DB helpers are still less ergonomic than ORMs like Prisma. We write more SQL by hand, which we view as a feature (see ADR-0007).
- Hiring pool in Thailand is smaller for Go than for JS/Python.

### Neutral
- Modular monolith inside a single repo: `internal/{auth,hotel,roomtype,landing,booking,pricing,notification,subscription}`. Each module owns its own Repository, Service, Handler, errors.
- Same binary serves API and worker via separate `cmd/` entry points.

## Alternatives considered

### Alt 1: Node.js (NestJS or Fastify) + Prisma
Rejected. Higher per-process memory (~150–300 MB idle), GC pauses under load, and Prisma's generated client is heavy. Type safety via TypeScript is good but pgx + hand-written SQL gives us more visibility into the FOR UPDATE patterns we depend on (ADR-0007).

### Alt 2: Python (FastAPI) + SQLAlchemy
Rejected. Async story is workable but the booking race patterns (ADR-0007) and the notification outbox (ADR-0008) lean heavily on raw SQL with FOR UPDATE / SKIP LOCKED — easier in Go with pgx than through SQLAlchemy.

## Notes

- See `apps/api/cmd/api/main.go` for wiring, `apps/api/internal/platform/server/` for the chi router tree.
- Generic helpers (`scanBooking`, `scanNotification`) keep query code close to SQL — no ORM tax.
