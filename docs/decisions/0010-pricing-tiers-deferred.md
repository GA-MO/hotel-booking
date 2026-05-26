# ADR-0010 — Pricing tiers deferred pending customer research

- **Status:** Accepted
- **Date:** 2026-05-26
- **Deciders:** team
- **Related:** plan.md §12 (Decisions Log — Pending), §1 (Business Model), §9 (Phase 0–1 roadmap), ADR-0001

## Context

ADR-0001 commits us to the subscription model. The natural follow-up is:
*what does each tier cost, and what differentiates them?* Plan.md §1
sketches a placeholder of Starter ~฿299–499, Growth ~฿699–999, Pro
~฿1,499–2,499 per hotel per month, and §12 lists this as explicitly
pending.

The competitive landscape gives us very little signal. Direct competitors
range from Sirvoy at roughly $9–29/mo to Cloudbeds at roughly $80–150/mo
— a 5–10× spread. They serve overlapping markets but make different bets
about feature gating, channel-manager inclusion, and per-room versus flat
pricing. Copying any one of them would either underprice us (and starve
us of revenue post-trial) or overprice us (and starve us of conversions
post-trial). And Thai small hotels' willingness-to-pay is its own animal —
not the same as US/EU operators despite running similar businesses.

We have no data. Picking a number now is a coin-flip dressed as a
decision.

## Decision

**Do not commit to specific tier prices, room-count breakpoints, or
feature-gating boundaries** in code or marketing copy. The codebase already
treats `subscription.plan_code` as a string (migration 0008), with all
features feature-flag-gated against the resolved plan. Tier definitions
live in configuration / database, not in conditionals scattered through
the code.

Locking the prices is gated on:

1. 10–15 interviews with Thai small-hotel operators (Phase 0, in parallel
   with engineering).
2. A competitor pricing audit.
3. A landing-page A/B with intent-capture forms during Phase 0–1, before
   the first paying cohort hits month 12 of trial.

## Consequences

### Positive
- We do not anchor on a number we will regret. If interviews tell us "฿599 is the magic number," we ship that; if they say "annual prepay only," we ship that instead.
- Tier shape is portable. A new plan is `INSERT INTO plans …` + a feature-flag update — no redeploy of application code.
- Marketing copy stays generic ("from ฿299/mo") until we have conviction. We do not commit to numbers that we then walk back publicly.

### Negative / trade-offs
- **Sales conversations have no firm answer to "how much will this cost when the trial ends?"** We mitigate with a published price range and a guarantee of 60 days' notice before any tier becomes effective.
- Feature-flag-driven tiers add one layer of indirection compared to hard-coded `if plan == "pro"` checks. We view this as a feature, not a tax.
- The marketing landing page must do double-duty as a price-discovery instrument (intent capture, WTP surveys) — that's extra work.

### Neutral
- Stripe / Omise product+price objects can be created lazily once tiers are locked. Until then, every account is in `trialing` (plan.md §6.5) and no real billing happens.

## Alternatives considered

### Alt 1: Pick numbers now and iterate
Rejected. We would be picking blind and then walking back publicly when interviews contradict the guess. Worse: an underpriced tier is essentially impossible to raise without churning the customers we want most.

### Alt 2: Copy a competitor's tiers exactly (e.g. Sirvoy)
Rejected. Their cost structure and customer mix differ from ours, and "we are exactly Sirvoy but in Bangkok" is not a position. Also legally on the edge if we lifted descriptions verbatim.

### Alt 3: One flat price for all hotels
Considered. Attractively simple but ignores the obvious WTP signal — a 5-room B&B and a 40-room boutique cannot rationally pay the same fixed price. Revisit if interviews surprise us.

## Notes

- Plan code lives at `subscription.plan_code`. The current default is `trial`.
- `subscription.unit_price` and `subscription.room_count` columns are already in place to support per-room pricing once we commit.
- Re-open this ADR (supersede, do not edit) once the interview round closes.
