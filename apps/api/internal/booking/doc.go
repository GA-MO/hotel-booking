// Package booking implements the reservation lifecycle: creation under a
// pessimistic lock (FOR UPDATE on the room_types row, ADR-0007) through
// confirmation, check-in, check-out, cancellation, and expiry of unpaid
// holds.
//
// State machine:
//
//   pending_payment → confirmed | expired | cancelled
//   confirmed       → checked_in | cancelled | no_show
//   checked_in      → checked_out
//   checked_out     → completed   (set externally; not yet wired)
//
// Transitions go through Repository methods that use conditional UPDATE
// (WHERE status = expectedFrom) so a concurrent transition can't apply
// twice. The append-only `booking_events` table records every change
// for audit + replay.
//
// Money: int64 minor units (satang). The booking row carries a *snapshot*
// of price breakdown captured at creation — subsequent rule edits do not
// retroactively change a booking's total. References are 9-char codes
// "HB-XXXXXX" using a 28-char alphabet that omits vowels and visually
// ambiguous characters; ~480M values with collision-detected insert.
//
// Race condition test in integration_test.go fires 10 concurrent goroutines
// against an inventory of 3 and verifies exactly 3 succeed. Don't refactor
// away the FOR UPDATE lock without proving an alternative passes that test.
//
// External effects (notifications, analytics, webhooks) fire through
// [Service.SetEventHook] so this package has no import-time dependency on
// notification, billing, etc. — wire the hook in cmd/api or server.New.
package booking
