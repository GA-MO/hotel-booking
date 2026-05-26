# Architecture Decision Records

Short, dated records of decisions that are **non-obvious** or **counter-intuitive** — the kind a future contributor (human or AI) might be tempted to re-litigate. We **don't** ADR-ify stack picks or industry-standard practices; that bloats the directory and dilutes signal.

For everything else — strategic positioning, stack choices, hosting, pricing tiers — the canonical source is [`plan.md`](../../plan.md), especially **§12 Decisions Log**.

## Current ADRs

| # | Title | Why this is an ADR |
|---|---|---|
| [0006](0006-money-as-int64-minor-units.md) | Money as int64 minor units | Counter-intuitive (most defaults to float / NUMERIC) |
| [0007](0007-select-for-update-booking-race.md) | SELECT FOR UPDATE for booking race | Subtle correctness; easy to refactor away without realising |
| [0008](0008-outbox-pattern-notifications.md) | Outbox pattern for notifications | Architectural; choosing wrong is hard to back out of |
| [0009](0009-structured-template-not-page-builder.md) | Structured template, not page builder | Goes against UX instinct (more flexibility ≠ better) |

## Adding a new ADR

Only add an ADR when the decision meets **all three** criteria:

1. **Non-obvious** — a reasonable engineer might genuinely choose otherwise.
2. **Hard to reverse** — changing it later costs more than writing the ADR.
3. **Not already covered** in `plan.md` or commit-level notes.

If it fails any of those, write it in a PR description or `plan.md §12` instead.

**Format:** [MADR](https://adr.github.io/madr/) template (see existing ADRs). Status starts at `Proposed` when opened in a PR; flip to `Accepted` on merge.
**Numbering:** Continue the sequence (next is 0011). Don't re-use the deleted numbers 0001–0005, 0010.
**Superseding:** When an ADR is overturned, set its status to `Superseded by ADR-NNNN` and add a top-of-file note. Don't delete history.

## History

The first round of ADRs (0001–0010, written 2026-05-26) included six entries that turned out to fail the three-criteria test — they restated decisions already in `plan.md` (positioning, hosting, stack picks) or industry-standard practices (argon2id, deferred pricing) without adding non-obvious context. Those have been removed in favour of pointing at `plan.md §12`. The four kept here genuinely warrant the ceremony.
