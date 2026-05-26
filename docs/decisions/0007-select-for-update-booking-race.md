# ADR-0007 — SELECT FOR UPDATE to prevent the booking race

- **Status:** Accepted
- **Date:** 2026-05-26
- **Deciders:** team
- **Related:** plan.md §6.3 (Race Condition Prevention), §10.3 (Race Conditions), code: `apps/api/internal/booking/repository.go` (`CreatePending`)

## Context

Concurrency on inventory is the single most dangerous correctness bug in a
booking system. Two guests hit "Book now" simultaneously on the last
available room; both requests pass the availability check; both inserts
succeed; the hotel is now oversold and has to call one guest to apologise.
That is the bug we must not ship.

Our inventory model is per-`(room_type, date)`. A booking spans one or
more nights, and "available" for a night means
`total_inventory + override.inventory_change − sum(room_count of active
overlapping bookings)`. "Active" means status ∈ {`pending_payment`,
`confirmed`, `checked_in`}.

The classic anti-patterns:

- **Check-then-insert with no lock.** Time-of-check / time-of-use race; trivially overbooks.
- **App-level mutex.** Works only inside one process. The moment we run two API replicas (which we do under Caddy), it's defunct.
- **Distributed lock (Redis Redlock).** Plausible but adds a second system in the booking-create critical path. Wrong order of dependencies for our scale.

The options that *do* work:

1. **Pessimistic lock**: `SELECT … FOR UPDATE` on a row that all concurrent attempts have to lock — typically the `room_types` row, since every booking for a given room type must serialise against the same parent record.
2. **Optimistic concurrency**: a `version` column on `room_types` that the booking insert CAS-checks; retry on conflict.
3. **`SERIALIZABLE` isolation**: lets Postgres detect the conflict and abort one transaction; we retry in application code.

## Decision

Use **pessimistic locking**: `BEGIN`, `SELECT total_inventory FROM
room_types WHERE id = $1 FOR UPDATE`, then count active bookings per night
in the *same* transaction, then `INSERT bookings (status='pending_payment',
expires_at=NOW()+10min)`, then `COMMIT`. A worker sweeps `pending_payment`
rows whose `expires_at` has lapsed (status → `expired`).

Validated by `TestRepo_CreatePending_RaceNoOversell` in
`internal/booking/integration_test.go`: 10 concurrent goroutines all
attempting to book against a `total_inventory=3` room. Exactly 3 succeed
and 7 return `ErrNoAvailability`. The test reliably fails without
`FOR UPDATE`.

## Consequences

### Positive
- Correctness is provable and locally reasoned — the lock is held for the duration of the booking-create transaction, period.
- No retry storm under contention; the second transaction simply blocks on `pg_locks` until the first commits.
- Survives multiple API replicas with zero extra coordination — Postgres is the source of truth.
- The 10-minute hold pattern (`pending_payment` + `expires_at`) gives the guest time to complete payment without blocking inventory permanently.

### Negative / trade-offs
- Lock contention scales linearly with concurrent attempts on the *same* `room_type`. Empirically fine up to ~50 QPS on a single room type — beyond that, the lock becomes the bottleneck.
- A pathologically slow booking transaction (e.g., a 3-second JSON encode bug) would back up every other concurrent booking on the same room type. We mitigate by keeping the transaction body tight — no external HTTP calls inside.
- `ReadCommitted` isolation is enough here because we hold an explicit row lock; we are not relying on snapshot isolation for correctness.

### Neutral
- Worker (`cmd/worker`) sweeps expired holds every 60 s and once on boot. plan.md §6.3.

## Alternatives considered

### Alt 1: Optimistic concurrency (`version` column + CAS retry)
Rejected for Phase 1. More moving parts (retry budget, jittered backoff, careful idempotency), and the win — lower contention under load — does not apply at our scale. Reconsider if/when a single room type's QPS climbs into the hundreds.

### Alt 2: PostgreSQL advisory locks
Rejected. Equivalent semantics for our purposes but disconnected from the row being protected, so misuse is easier (lock key mismatches with the row update) and harder to audit.

### Alt 3: Redis distributed lock
Rejected. Adds Redis to the booking-create critical path, with all the operational hazards (split-brain on failover, clock skew on Redlock). Postgres is already in the critical path — adding Redis is gratuitous.

### Alt 4: App-level mutex
Rejected. Does not survive multi-instance deployment, which is our Phase 1 default behind Caddy.

## Notes

- Idempotency key on `POST /bookings` (plan.md §10.3) is a separate concern handled at the HTTP layer; this ADR is about the DB-level race only.
- If we ever need higher single-room-type throughput, the migration path is documented in plan.md §10.3 (Redis distributed lock).
