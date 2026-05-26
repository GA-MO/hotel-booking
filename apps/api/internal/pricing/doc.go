// Package pricing implements daily availability overrides, rule-based
// pricing modifiers (season, day-of-week, length-of-stay, advance-purchase),
// and a pure-Go engine that computes a per-night quote.
//
// Money: int64 in minor units (satang for THB) throughout the engine
// (ADR-0006). The JSON boundary formats as decimal strings ("1500.00")
// to dodge JS float precision. Modifier values are stored as either
// percentage (in hundredths-of-percent so applyModifier stays integer),
// fixed-amount (cents), or set-value (cents).
//
// Engine evaluation order — see plan.md §8:
//
//  1. base_rate from room_type
//  2. day_of_week rules (e.g. weekend +20%) compounded by priority
//  3. season rules compounded by priority
//  4. date-specific override (availability_overrides.rate_override)
//     REPLACES the computed rate for that night
//  5. sum per-night rates → subtotal
//  6. length_of_stay modifier on the subtotal
//  7. advance_purchase modifier on the subtotal
//
// The /v1/public/quote/{slug} endpoint is a *preview* — it does NOT
// reserve inventory or check availability. The booking module's
// CreatePending is the only path that subtracts inventory under a row
// lock (ADR-0007).
package pricing
