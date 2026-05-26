# ADR-0006 — Money stored as int64 minor units

- **Status:** Accepted
- **Date:** 2026-05-26
- **Deciders:** team
- **Related:** plan.md §8 (Pricing & Availability), code: `apps/api/migrations/0006_bookings.up.sql`, `apps/api/internal/pricing/`

## Context

Money appears in many places in the system: room base rates, pricing rule
modifiers (percentage or fixed), tax and fee lines, per-night and per-stay
totals, refund and partial-refund amounts, exchange-rate snapshots, and
subscription invoices. Two facts make this non-trivial:

1. **Float arithmetic is unsound for money.** `0.1 + 0.2 != 0.3` in IEEE 754,
   and the failure mode is silent rounding drift across totals — exactly
   the bug that triggers customer-facing reconciliation incidents.
2. **The pricing engine is multi-step.** plan.md §8 describes a 10-stage
   pipeline (base → DoW → season → override → per-night → LOS → extras →
   taxes → fees → promo). Errors compound across stages.

The three sensible representations are:

- `NUMERIC(p, s)` in Postgres + a decimal library in Go (e.g. `shopspring/decimal`).
- `int64` minor units (cents/satang) end-to-end.
- `float64` (ruled out on the first paragraph).

## Decision

All money is stored and arithmetic-ed as **`int64` in the smallest currency
unit** (satang for THB, cents for USD). Database columns are `BIGINT`
suffixed `_cents` (e.g. `room_subtotal_cents`, `taxes_cents`, `total_cents`
in migration 0006). The pricing engine is pure `int64`. APIs emit money as
decimal strings (e.g. `"1500.00"`) at the boundary; clients never receive
raw cents. Exchange rates are stored as a snapshot integer numerator /
denominator pair on each booking, never as a float.

## Consequences

### Positive
- Addition, subtraction, and integer multiplication are exact. Totals reconcile bit-for-bit.
- `int64` holds up to ~9.2 × 10¹⁸ minor units — i.e. ~9 × 10¹⁶ THB — well above any realistic invoice or annual GMV.
- DB constraint `CHECK (total_cents >= 0)` is a one-liner; the same check on a NUMERIC needs trailing-zero awareness.
- Test assertions are exact (`assert total == 949050` satang) — no `WithinDelta`.

### Negative / trade-offs
- Cannot multiply directly by a float exchange rate. Any FX or percentage modifier (e.g. weekend +20 %) must round explicitly and document the rounding mode (we use banker's rounding via `(a*num + denom/2) / denom`).
- The API boundary must format minor units → decimal string and parse string → minor units. We have a single helper for this; misuse would be a footgun.
- Currencies with non-cent subunits (e.g. JPY has zero, BHD has thousandths) need a per-currency `minor_unit_exponent`. We hard-code 2 for now and revisit when we onboard a hotel that needs otherwise.

### Neutral
- `int64` columns are 8 bytes — the same as `NUMERIC(10, 2)` would index. No storage win or loss.

## Alternatives considered

### Alt 1: `NUMERIC(10, 2)` in Postgres + `shopspring/decimal` in Go
Rejected. Adds a third-party dep with its own arithmetic semantics; every cross-boundary conversion is a chance for drift; benchmark shows decimal multiplication ~3–5× slower than int64. Real benefit (representing fractional cents during intermediate rounding) is moot because we round at every step anyway.

### Alt 2: `float64`
Rejected. See first paragraph. This is a non-starter for money.

## Notes

- See `apps/api/migrations/0006_bookings.up.sql` for the canonical column shape.
- The pricing engine assertion in `internal/pricing/engine_test.go` for the canonical "weekend + season + LOS + AP" 7-night July combo locks the contract: exactly `949050` satang.
- One small footgun lives in `internal/booking/repository.go` (`GetRoomTypeBasics`) — `base_rate` is currently `NUMERIC(10,2)` and is converted to cents via `int64(rateNum*100 + 0.5)`. We should migrate `base_rate` to `_cents` and remove the float hop. Tracked as a follow-up.
