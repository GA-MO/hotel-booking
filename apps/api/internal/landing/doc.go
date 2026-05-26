// Package landing implements the per-hotel, per-locale landing page used
// by the booking-web Next.js app (rendered with ISR for sub-1.5s LCP).
//
// Content model: structured template, not free-form page builder
// (ADR-0009). Hotels customize branding (colors, logo, fonts from a
// curated set), section ordering, SEO meta, and tracking pixel IDs.
// The section type whitelist (hero | gallery | rooms | amenities | ...)
// prevents random JSON accumulating — unknown types are rejected at
// upsert. Section `content` is a JSONB blob, kept untyped here so the
// frontend can evolve the per-section shape without DB migrations.
//
// Two router methods:
//
//   - [Handler.Routes] — admin (PUT/POST/DELETE) at /v1/hotels/{id}/landing
//   - [Handler.PublicRoutes] — anonymous GET at /v1/public/landing/{slug}/{locale}
//
// The public endpoint only returns rows where the parent hotel is `live`
// AND the landing page is `published`; drafts are intentionally
// undiscoverable to prevent leaking unfinished marketing copy.
//
// Versioning: each upsert increments `version` and resets `status` to
// `draft`. The publish endpoint flips status without changing content.
package landing
