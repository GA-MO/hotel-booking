# Architecture Decision Records

This directory holds the Architecture Decision Records (ADRs) for the
hotel-booking SaaS. Each file captures one decision that shapes the system —
the context that forced the choice, the choice itself, the trade-offs we
accepted, and the alternatives we explicitly rejected.

For the wider product and architecture context that frames every ADR here,
see [`/plan.md`](../../plan.md) — particularly §1 (positioning), §2 (stack),
§6 (booking flow), §7 (landing builder), §8 (pricing), §10 (considerations),
§11 (anti-features), and §12 (decisions log).

## Format

We use the [MADR](https://adr.github.io/madr/) (Markdown ADR) template, kept
intentionally short. Each ADR has:

- **Status:** `Proposed` → `Accepted` → `Superseded by ADR-NNNN`
- **Date:** when the decision was accepted
- **Deciders:** who owned the call
- **Related:** plan sections and cross-referenced ADRs
- **Context / Decision / Consequences / Alternatives / Notes**

ADRs are immutable once `Accepted`. If a decision changes, add a new ADR
that supersedes the old one and update the old file's status. Do not
rewrite history.

## Current ADRs

| #    | Title                                                                                            | Status   |
| ---- | ------------------------------------------------------------------------------------------------ | -------- |
| 0001 | [Subscription, not commission](0001-subscription-not-commission.md)                              | Accepted |
| 0002 | [Direct booking, not marketplace](0002-direct-booking-not-marketplace.md)                        | Accepted |
| 0003 | [Self-host on Hetzner Cloud](0003-self-host-hetzner.md)                                          | Accepted |
| 0004 | [Go backend with chi + pgx + slog + envconfig](0004-go-backend-chi-pgx.md)                       | Accepted |
| 0005 | [argon2id for password hashing](0005-argon2id-passwords.md)                                      | Accepted |
| 0006 | [Money stored as int64 minor units](0006-money-as-int64-minor-units.md)                          | Accepted |
| 0007 | [SELECT FOR UPDATE to prevent booking race](0007-select-for-update-booking-race.md)              | Accepted |
| 0008 | [Outbox pattern for notifications](0008-outbox-pattern-notifications.md)                         | Accepted |
| 0009 | [Structured template, not free-form page builder](0009-structured-template-not-page-builder.md)  | Accepted |
| 0010 | [Pricing tiers deferred pending customer research](0010-pricing-tiers-deferred.md)               | Accepted |

## Adding a new ADR

1. Copy the template from any existing ADR.
2. Pick the next free number (zero-padded to four digits).
3. Filename: `NNNN-short-kebab-slug.md`.
4. Start with `Status: Proposed` while it's open for discussion.
5. Flip to `Status: Accepted` once the team has signed off; commit.
6. Add a row to the table above.
7. If this ADR supersedes another, update the old ADR's status to
   `Superseded by ADR-NNNN` and link both ways.

Keep ADRs short (50–120 lines). The point is to capture *why*, not to be a
spec — implementation details belong in code and in `plan.md`.
