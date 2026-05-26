# ADR-0001 — Subscription, not commission

- **Status:** Accepted
- **Date:** 2026-05-26
- **Deciders:** team
- **Related:** plan.md §1 (Vision & Positioning), §6.4 (Payment Options), §12 (Decisions Log), ADR-0002

## Context

OTAs (Booking.com, Agoda, Expedia) charge small hotels 15–25 % commission on
every booking. That model is great for the OTA — they take a slice of every
transaction the hotel makes through them — but it is the single biggest
operating cost line for the boutique / B&B / hostel / guesthouse operators
we are targeting. Any tool that offers to displace OTA bookings has to be
visibly cheaper than the OTA cut, otherwise the hotel has no reason to switch.

We can monetise this in one of three ways: take our own commission per
booking, charge a flat subscription, or some hybrid. Commission has the
attraction of "free to start" but it forces us into the payment flow — we
would be acting as a payment facilitator, which in Thailand requires a
license from the Bank of Thailand and a non-trivial AML/KYC program. It
also makes our revenue volatile and ties our incentives to charging the
hotel more, which is the opposite of our pitch.

## Decision

Charge hotels a flat monthly subscription per hotel (target range
฿299–1,499/mo, tier shape TBD per ADR-0010). The guest's money flows
directly from guest to hotel via the hotel's own gateway (PromptPay,
Omise, Stripe). We never touch guest funds. First 365 days are free as
an acquisition lever.

## Consequences

### Positive
- No payment-facilitator license needed — the hotel is the merchant of record.
- Revenue is predictable and grows linearly with active hotels.
- Incentives align: we win when hotels stay alive and grow, not when guests pay more.
- The pitch line writes itself: "no 15–25 % to OTAs, just ฿X/mo."

### Negative / trade-offs
- Slow initial revenue ramp — the free year means months 0–12 are pure burn.
- We must build a real dunning / subscription lifecycle (see plan.md §6.5) before trial cohorts churn.
- We collect zero data on guest payment patterns (since we do not see the transactions).

### Neutral
- Subscription billing itself still runs through Stripe + Omise — these are *our* merchants, not the hotel's.

## Alternatives considered

### Alt 1: Pure per-booking commission
Rejected. Requires payment facilitator status (BoT license + AML), volatile
revenue, and worst of all our pitch becomes "pay us instead of the OTA" —
identical economics to what we're displacing.

### Alt 2: Hybrid (small subscription + small per-booking fee)
Rejected for Phase 1. Doubles the billing surface area, confuses the value
prop, and re-introduces the facilitator question. Can be revisited post-PMF
if we want a usage-based upgrade lane on top of tiers.

## Notes

- Tier amounts and feature gating decided in ADR-0010 after customer research.
- Pre-launch payment-method capture flow is explicitly Phase 2 — see plan.md §9.
