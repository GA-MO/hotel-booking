// Package roomtype implements room-type CRUD plus hotel-level and
// room-type-level photo attachments.
//
// Inventory model: each room_type has a `total_inventory` integer — the
// number of physical rooms of this type. Daily availability is derived
// at read time from total_inventory + any `availability_overrides`
// (managed by the pricing package) minus the count of active bookings
// overlapping each date.
//
// Photos: storage-key references only. The actual upload pipeline
// (presigned URL → MinIO/B2 → imgproxy delivery) is Phase 2 — Phase 1
// records the storage key once the client has uploaded out-of-band.
// At most one cover photo per parent (enforced by a partial UNIQUE
// index on `is_cover = TRUE`).
//
// All endpoints are auth-gated and account-scoped: cross-tenant access
// returns 404, never 403.
package roomtype
