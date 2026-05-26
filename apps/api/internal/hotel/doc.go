// Package hotel implements per-property CRUD: signup → KYC → live. A hotel
// belongs to exactly one account; multi-property hotels under the same
// account are supported, though the current admin UI assumes one.
//
// Slug uniqueness is enforced by a Postgres UNIQUE constraint. The
// [Repository.SlugAvailable] check is advisory only — the authoritative
// answer is the INSERT, which surfaces as [ErrSlugAlreadyTaken] on
// constraint violation. Don't gate signup UX on the advisory check alone.
//
// Tenant isolation: every query is filtered by `account_id` from the
// authenticated [auth.Identity]. Cross-account access returns
// [ErrHotelNotFound] (404) rather than 403 to avoid disclosing the
// existence of other tenants' rows.
//
// `status` lifecycle: test → live → suspended | archived. Only `live`
// hotels accept public bookings (`/v1/public/hotels/{slug}/bookings`);
// staff walk-in bookings can be recorded against test-mode hotels too.
//
// `promptpay_id` is optional. When present it must match one of three
// PromptPay receiver formats: 10-digit MSISDN (mobile), 13-digit Thai
// national-ID, or 15-char e-Wallet / merchant tax-ID. Format validation
// runs in [Service] (see `normalisePromptPayID`); the DB column has no
// CHECK so the rules can evolve without a migration. PATCH semantics are
// tri-state: nil = unchanged, "" = clear, non-empty = set.
package hotel
