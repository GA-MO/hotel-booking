# ADR-0009 — Structured template, not free-form page builder

- **Status:** Accepted
- **Date:** 2026-05-26
- **Deciders:** team
- **Related:** plan.md §7 (Landing Page Builder), §10.1 (Speed = Revenue), §11 (Anti-features)

## Context

Every hotel needs a landing page at `book.[platform].com/[hotel-slug]`.
The page is the funnel — it converts ad-clicks into bookings. Plan.md
§10.1 says it clearly: if LCP creeps above 2.5 s, conversion drops 7–15 %
and the hotel's ad budget is wasted. Whatever we ship must be fast on a
Thai mobile carrier on a mid-range Android device.

The product question is: how much creative control does the hotel get?
The market has two extremes:

- **Free-form page builder** (Webflow, Wix, Squarespace, Elementor):
  drag-and-drop blocks, arbitrary CSS/JS, infinite layouts. Power user
  paradise. Non-designer disaster: most non-designer-built pages are
  visually broken, slow, and convert worse than a generic template.
- **Hard-coded page**: one layout, content swap only. Fast, ugly across
  hotels that look different.

Our customer is a small-hotel operator (boutique, B&B, hostel, guesthouse)
who is *not* a designer and *should not* be one. They want a page that
converts; they do not want a creative tool.

## Decision

Landing pages are a **structured template with a content catalogue**.
Sections come from a fixed, whitelisted catalogue (Phase 1: Hero, Gallery,
About, Room Types, Amenities, Location, Reviews, FAQ, Policies, Contact).
Hotels choose which sections to enable, their order, and the content of
each (text, photos, brand colour, logo, SEO meta, tracking IDs). They do
**not** get custom HTML, CSS, JavaScript, or font choices outside our
preset 4–5 typefaces. The page is rendered with Next.js ISR + Cloudflare
edge cache.

## Consequences

### Positive
- **Conversion-optimised layout that we own end-to-end.** When we A/B-test the hero, every hotel benefits. When we tune the room-card CTA, every hotel benefits.
- **LCP target is achievable.** No builder runtime, no third-party CSS-in-JS, no arbitrary fonts. ISR-rendered HTML at the edge → LCP < 1.5 s on 4G mobile is the design target (plan.md §10.1).
- **Mobile responsiveness is our problem, not the hotel's.** A single layout, tested across breakpoints, never breaks on a phone because a hotel dragged a column too wide.
- **Backend shape is simple.** `landing_pages.sections` is a JSONB array of typed objects (see plan.md §5.4). The renderer pattern-matches `section.type` against the whitelisted catalogue.
- **Security.** No arbitrary JS → no XSS by hotel content. No custom CSS → no `position: fixed` overlays hijacking the booking CTA.

### Negative / trade-offs
- A hotel with a strong existing brand or an unusual layout (e.g., a longform-narrative boutique) will feel constrained. We accept that — they are not our target segment yet.
- Adding a new section type is an engineering task, not a self-serve one. We cannot ship "an aquarium widget" because one hotel wants it. We add sections via the catalogue based on observed demand.
- We carry the design-quality burden. If our template is ugly, every hotel looks ugly.

### Neutral
- Multi-language is per-(hotel, locale, version). TH/EN in Phase 1; auto-translate suggestion in Phase 2 (plan.md §7).
- Tracking integration (FB Pixel, GA, GTM, LINE Tag, TikTok Pixel) is built into the renderer with PDPA/GDPR consent gating before any pixel fires.

## Alternatives considered

### Alt 1: Free-form WYSIWYG page builder
Rejected. Known to produce bad output by non-designers; runtime cost crushes LCP; opens an XSS / CSP surface; we can no longer A/B-test cross-hotel; ranking algorithms (we have none, but Google's Page Experience signals) penalise the result.

### Alt 2: Headless CMS (Sanity / Contentful / Strapi)
Rejected. Overkill for our content shape — every section field is enumerated and lives perfectly well in a JSONB column. A CMS adds an external system to the critical path and a separate auth/permissions model.

### Alt 3: Static, single-template, content-swap only (no section reorder, no toggles)
Rejected. Too restrictive — different hotel types (hostel vs B&B vs boutique) genuinely need different section emphasis. Toggling and reordering within a fixed catalogue is the minimum viable flexibility.

## Notes

- Section catalogue is defined in `apps/api/internal/landing/`. New sections require a code change (Go validator + Next.js renderer).
- See plan.md §11 — "free-form page builder (custom CSS/JS)" is an explicit anti-feature.
