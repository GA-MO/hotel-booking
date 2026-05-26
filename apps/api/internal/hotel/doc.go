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
package hotel
