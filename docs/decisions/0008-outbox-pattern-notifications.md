# ADR-0008 — Outbox pattern for notifications

- **Status:** Accepted
- **Date:** 2026-05-26
- **Deciders:** team
- **Related:** plan.md §10.7 (Failure Modes), §3 (Architecture), code: `apps/api/internal/notification/repository.go` (`ClaimBatch`), ADR-0007

## Context

When a booking is created, confirmed, or cancelled, we owe the guest an
email (and eventually a LINE message) and the hotel staff a notification.
The naive way is to call Resend / LINE / SMS inline inside the
booking-create handler. That has three failure modes:

1. **Provider slowness or outage** stretches the request that the guest sees. Resend's p99 can hit several seconds; a LINE outage can hit minutes.
2. **Provider error becomes a booking error.** If Resend returns 5xx, we either roll back the booking (worst — the guest paid but we did not book them) or commit but lose the notification (also bad).
3. **No retry surface.** Inline sends mean a single transient failure is permanent — there is no row anywhere to retry from.

Plus the booking-create transaction is already holding a `FOR UPDATE` lock
on `room_types` (ADR-0007). Stretching that transaction with an HTTP call
to Resend is unacceptable — every concurrent booking attempt on the same
room type would block on it.

## Decision

Use the **outbox pattern**. Domain modules (booking, subscription) enqueue
notification rows into the `notifications` table inside the same DB
transaction that performs the domain change. A separate worker
(`cmd/worker`) drains the outbox.

The drain query uses a CTE + `FOR UPDATE SKIP LOCKED`:

```sql
WITH claimed AS (
  SELECT id AS claimed_id FROM notifications
  WHERE status = 'queued' AND send_after <= $now
  ORDER BY send_after
  FOR UPDATE SKIP LOCKED
  LIMIT $batch
)
UPDATE notifications SET status='sending', attempts = attempts + 1
FROM claimed WHERE notifications.id = claimed.claimed_id
RETURNING …
```

Retry policy: exponential backoff (2/4/8/16/32 min), `MaxAttempts = 5`,
dead-letter on permanent render errors and after the cap.

## Consequences

### Positive
- **Booking-create returns fast** — the only DB write is one row in `notifications`. No external HTTP in the transaction.
- **Atomic with the domain event.** The notification row commits together with the booking; either both land or neither does. No "booked but never notified" / "notified but never booked" inconsistency.
- **Retry surface for free.** Every notification has a row, an attempt count, a last error, and a `send_after`. We can debug, replay, dead-letter.
- **`SKIP LOCKED` lets workers scale horizontally** — multiple worker replicas can drain the same outbox without ever blocking each other or claiming the same row.
- **Provider-agnostic at the row level.** `channel` is one of `email | line | sms`; `template` selects the renderer; the `Sender` interface routes per channel.

### Negative / trade-offs
- Delivery is **eventually consistent** — a guest may see "booking confirmed" on screen before the email lands. Acceptable; we set the expectation in the UI copy.
- **Extra DB write per notification.** At our scale this is rounding error vs the booking write itself.
- A render-time bug (template that panics) currently dead-letters immediately on the first attempt. Intentional — retrying a deterministic bug helps nobody — but it does mean a bad template is a paged incident, not a slow one.
- The worker is now a piece of stateful infrastructure. If it stops, notifications back up.

### Neutral
- Tick interval is 60 s. The first send happens within ~60 s of enqueue — acceptable for booking confirmations; tunable later if needed.
- Same pattern is used by `subscription.RunMaintenance` for state transitions (trialing → trial_ending, etc.) — see `internal/subscription/`.

## Alternatives considered

### Alt 1: In-process queue (Go channel + goroutine)
Rejected. Lost on crash, lost on deploy, lost on OOM. No retry surface. Survives only the lifetime of the process.

### Alt 2: Redis stream or RabbitMQ
Rejected. Adds a second system to the booking-create transaction; you either lose atomicity with the DB write or you implement two-phase commit. The whole point of the outbox is that the DB *is* the queue.

### Alt 3: Inline send from the request handler
Rejected. See "Context" above — couples request latency to provider latency, no retry, exposes the user to provider errors.

## Notes

- `cmd/worker/main.go` drives the drain on a 60-second tick.
- Render errors dead-letter immediately; send errors retry until `MaxAttempts`.
- The bugfix in commit `393646e` (CTE column rename `id → claimed_id`) is required because Postgres errors with 42702 on ambiguous `id` in the `RETURNING` clause — leave that rename in place.
