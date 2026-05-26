# ADR-0002 — Direct booking, not marketplace

- **Status:** Accepted
- **Date:** 2026-05-26
- **Deciders:** team
- **Related:** plan.md §1 (Vision & Positioning), §11 (Anti-features), ADR-0001

## Context

There are two very different products in the hotel software space, and they
get confused constantly. A *marketplace* (Agoda, Booking.com, Expedia) owns
the demand side: guests come to the marketplace to discover and compare
hotels, and the marketplace ranks results, mediates trust, and takes a
commission. A *direct booking engine* (Cloudbeds, Little Hotelier, Sirvoy,
SiteMinder Get Direct, HotelRunner) owns the supply side: the hotel uses
the engine to host its own booking page, and demand comes from the hotel's
own marketing — ads, social, repeat guests, walk-by signage.

Building a marketplace is enormously capital-intensive. It requires a
ranking algorithm, two-sided trust (reviews, guarantees, dispute resolution),
guest acquisition spend that scales with GMV, and a regulatory posture as a
payment facilitator. None of this is winnable as a small team against
incumbents that have spent a decade and billions on it.

Building a direct booking engine is a software product. The customer is the
hotel; the value is "your own conversion-optimised booking page + a PMS
underneath."

## Decision

Build a direct booking engine only. There is no cross-hotel discovery, no
search across the platform, no ranking, no "hotels near me," no guest-side
marketplace UX. Traffic enters at `book.[platform].com/[hotel-slug]` from
the hotel's own ads/social/email. Competitors are Cloudbeds, Little
Hotelier, Sirvoy — not Agoda or Booking.com.

## Consequences

### Positive
- Scope collapses dramatically — no ranking, no two-sided trust, no aggregated search index.
- Each hotel page is statically generatable (Next.js ISR + Cloudflare), so LCP < 1.5 s is achievable on commodity hardware.
- Hotels keep ownership of their customer relationship and guest data, which is the selling point against OTAs.
- We never need a recommendation system, no Elasticsearch / Meilisearch / ranking ML.

### Negative / trade-offs
- Hotels must drive their own traffic. We do not generate any. This is the largest objection in sales conversations.
- We cannot leverage cross-hotel data effects (recommendations, "trending in Chiang Mai", etc.).
- The product is invisible to guests who are not already on a hotel's funnel — there is no platform brand pull.

### Neutral
- Channel manager integration (Phase 3, plan.md §9) brings *outbound* sync to OTAs but still does not make us a marketplace.

## Alternatives considered

### Alt 1: Full marketplace (Agoda-style)
Rejected. Capital-intensive, requires a ranking algorithm, payment-facilitator
license, two-sided trust mechanisms, and a brand that takes years to build.

### Alt 2: Hybrid (direct engine + opt-in cross-hotel discovery)
Rejected for Phase 1. Adds an entire surface area (search, ranking, guest
accounts, cross-hotel reviews) before we have proven the simpler product
works. Could be revisited post-PMF if hotels explicitly ask for it.

## Notes

- See plan.md §11 for the explicit list of anti-features that fall out of this decision.
- The booking subdomain `book.[platform].com/{slug}` is the canonical entry point — Phase 3 adds custom domains (plan.md §12).
