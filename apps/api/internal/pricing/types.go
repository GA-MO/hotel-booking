package pricing

import (
	"time"

	"github.com/google/uuid"
)

// Date is a calendar day without timezone, stored/serialised as YYYY-MM-DD.
// We use a thin wrapper around time.Time so JSON encoding stays YYYY-MM-DD
// instead of the default RFC3339.
type Date struct {
	time.Time
}

// DateLayout is the canonical wire format for Date.
const DateLayout = "2006-01-02"

// ParseDate parses a YYYY-MM-DD string into a Date at UTC midnight.
func ParseDate(s string) (Date, error) {
	t, err := time.Parse(DateLayout, s)
	if err != nil {
		return Date{}, ErrInvalidDate
	}
	return Date{t}, nil
}

// MarshalJSON renders the date as "YYYY-MM-DD".
func (d Date) MarshalJSON() ([]byte, error) {
	if d.IsZero() {
		return []byte(`null`), nil
	}
	b := make([]byte, 0, 12)
	b = append(b, '"')
	b = append(b, []byte(d.Format(DateLayout))...)
	b = append(b, '"')
	return b, nil
}

// UnmarshalJSON parses a "YYYY-MM-DD" string into Date.
func (d *Date) UnmarshalJSON(b []byte) error {
	if len(b) >= 2 && b[0] == '"' && b[len(b)-1] == '"' {
		b = b[1 : len(b)-1]
	}
	if len(b) == 0 || string(b) == "null" {
		return nil
	}
	t, err := time.Parse(DateLayout, string(b))
	if err != nil {
		return ErrInvalidDate
	}
	d.Time = t
	return nil
}

// Equal reports whether two dates fall on the same calendar day (UTC).
func (d Date) Equal(other Date) bool {
	return d.Year() == other.Year() && d.YearDay() == other.YearDay()
}

// AvailabilityOverride captures the admin-set per-date inventory/rate/restriction
// deviation from the room_type defaults. Stored sparsely.
type AvailabilityOverride struct {
	HotelID         uuid.UUID `json:"hotel_id"`
	RoomTypeID      uuid.UUID `json:"room_type_id"`
	Date            Date      `json:"date"`
	InventoryChange int       `json:"inventory_change"`
	Closed          bool      `json:"closed"`

	// RateOverrideMinor is in minor units (e.g. THB satang). Pointer to allow null.
	RateOverrideMinor *int64  `json:"-"`
	RateOverride      *string `json:"rate_override,omitempty"` // decimal string for the wire
	RateCurrency      *string `json:"rate_currency,omitempty"`

	MinNights         *int `json:"min_nights,omitempty"`
	MaxNights         *int `json:"max_nights,omitempty"`
	ClosedToArrival   bool `json:"closed_to_arrival"`
	ClosedToDeparture bool `json:"closed_to_departure"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// PricingRule is a single modifier applied during rate computation.
type PricingRule struct {
	ID         uuid.UUID  `json:"id"`
	HotelID    uuid.UUID  `json:"hotel_id"`
	RoomTypeID *uuid.UUID `json:"room_type_id,omitempty"`

	Name     string `json:"name"`
	RuleType string `json:"rule_type"`

	StartDate  *Date `json:"start_date,omitempty"`
	EndDate    *Date `json:"end_date,omitempty"`
	DaysOfWeek []int `json:"days_of_week,omitempty"` // ISO weekday 1..7 Mon..Sun

	ModifierType  string `json:"modifier_type"`
	ModifierValue string `json:"modifier_value"` // decimal string

	MinNights    *int `json:"min_nights,omitempty"`
	MaxNights    *int `json:"max_nights,omitempty"`
	MinDaysAhead *int `json:"min_days_ahead,omitempty"`
	MaxDaysAhead *int `json:"max_days_ahead,omitempty"`

	Priority int  `json:"priority"`
	Enabled  bool `json:"enabled"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Rule types (mirrors the DB CHECK constraint).
const (
	RuleTypeSeason          = "season"
	RuleTypeDayOfWeek       = "day_of_week"
	RuleTypeLengthOfStay    = "length_of_stay"
	RuleTypeAdvancePurchase = "advance_purchase"
)

// Modifier types.
const (
	ModifierPercentage  = "percentage"
	ModifierFixedAmount = "fixed_amount"
	ModifierSetValue    = "set_value"
)

// DailyRate is a per-night rate breakdown returned by the engine.
// AmountMinor is in minor units (e.g. satang); Amount is the decimal-string
// representation we send over the wire to keep JS clients precise.
type DailyRate struct {
	Date        Date     `json:"date"`
	AmountMinor int64    `json:"-"`
	Amount      string   `json:"amount"`
	Currency    string   `json:"currency"`
	AppliedRule []string `json:"applied_rules,omitempty"`
}

// Quote is the engine's return value.
type Quote struct {
	Nights        int         `json:"nights"`
	Currency      string      `json:"currency"`
	PerNight      []DailyRate `json:"per_night"`
	SubtotalMinor int64       `json:"-"`
	Subtotal      string      `json:"subtotal"`
	TotalMinor    int64       `json:"-"`
	Total         string      `json:"total"`
	Adjustments   []string    `json:"adjustments,omitempty"` // human-readable trace of LOS/AP applied
}

// UpsertAvailabilityRequest is the body for PUT /v1/hotels/{hotel_id}/availability.
type UpsertAvailabilityRequest struct {
	Items []UpsertAvailabilityItem `json:"items"`
}

// UpsertAvailabilityItem represents a single (room_type_id, date) override row
// to insert or update.
type UpsertAvailabilityItem struct {
	RoomTypeID        uuid.UUID `json:"room_type_id"`
	Date              Date      `json:"date"`
	InventoryChange   int       `json:"inventory_change,omitempty"`
	Closed            bool      `json:"closed,omitempty"`
	RateOverride      *string   `json:"rate_override,omitempty"` // decimal string ("1500.00")
	RateCurrency      *string   `json:"rate_currency,omitempty"`
	MinNights         *int      `json:"min_nights,omitempty"`
	MaxNights         *int      `json:"max_nights,omitempty"`
	ClosedToArrival   bool      `json:"closed_to_arrival,omitempty"`
	ClosedToDeparture bool      `json:"closed_to_departure,omitempty"`
}

// CreateRuleRequest is the body for POST /v1/hotels/{hotel_id}/pricing-rules.
type CreateRuleRequest struct {
	RoomTypeID    *uuid.UUID `json:"room_type_id,omitempty"`
	Name          string     `json:"name"`
	RuleType      string     `json:"rule_type"`
	StartDate     *Date      `json:"start_date,omitempty"`
	EndDate       *Date      `json:"end_date,omitempty"`
	DaysOfWeek    []int      `json:"days_of_week,omitempty"`
	ModifierType  string     `json:"modifier_type"`
	ModifierValue string     `json:"modifier_value"`
	MinNights     *int       `json:"min_nights,omitempty"`
	MaxNights     *int       `json:"max_nights,omitempty"`
	MinDaysAhead  *int       `json:"min_days_ahead,omitempty"`
	MaxDaysAhead  *int       `json:"max_days_ahead,omitempty"`
	Priority      *int       `json:"priority,omitempty"`
	Enabled       *bool      `json:"enabled,omitempty"`
}

// UpdateRuleRequest is the body for PATCH /v1/hotels/{hotel_id}/pricing-rules/{id}.
// All fields optional; only non-nil are written.
type UpdateRuleRequest struct {
	Name          *string  `json:"name,omitempty"`
	StartDate     *Date    `json:"start_date,omitempty"`
	EndDate       *Date    `json:"end_date,omitempty"`
	DaysOfWeek    *[]int   `json:"days_of_week,omitempty"`
	ModifierType  *string  `json:"modifier_type,omitempty"`
	ModifierValue *string  `json:"modifier_value,omitempty"`
	MinNights     *int     `json:"min_nights,omitempty"`
	MaxNights     *int     `json:"max_nights,omitempty"`
	MinDaysAhead  *int     `json:"min_days_ahead,omitempty"`
	MaxDaysAhead  *int     `json:"max_days_ahead,omitempty"`
	Priority      *int     `json:"priority,omitempty"`
	Enabled       *bool    `json:"enabled,omitempty"`
}

// QuoteRequest is the body for POST /v1/public/quote/{slug}.
type QuoteRequest struct {
	RoomTypeID uuid.UUID `json:"room_type_id"`
	CheckIn    Date      `json:"check_in"`
	CheckOut   Date      `json:"check_out"`
	Rooms      int       `json:"rooms"`
}

// QuoteResponse is the wire form of an engine Quote with rooms multiplier applied.
//
// AvailableRooms is the minimum nightly availability over the requested stay
// (post-overrides, post-active-bookings). It is a *preview* — the booking
// module still re-checks under FOR UPDATE at commit time, so a non-zero value
// is not a reservation. The FE uses it to disable the submit button when
// AvailableRooms < Rooms and to render "X rooms left" / "sold out" copy.
// Closed reports whether the hotel has marked any day in the range as closed.
type QuoteResponse struct {
	HotelID        uuid.UUID   `json:"hotel_id"`
	RoomTypeID     uuid.UUID   `json:"room_type_id"`
	Rooms          int         `json:"rooms"`
	Nights         int         `json:"nights"`
	Currency       string      `json:"currency"`
	PerNight       []DailyRate `json:"per_night"`
	Subtotal       string      `json:"subtotal"`
	Total          string      `json:"total"`
	AvailableRooms int         `json:"available_rooms"`
	Closed         bool        `json:"closed,omitempty"`
	Adjustments    []string    `json:"adjustments,omitempty"`
}

// AvailabilityDay is one entry returned by GET /availability — the computed
// rate (post-rules, post-overrides) plus inventory/closure state for that day.
type AvailabilityDay struct {
	Date           Date     `json:"date"`
	RoomTypeID     uuid.UUID `json:"room_type_id"`
	TotalInventory int      `json:"total_inventory"`
	Available      int      `json:"available"` // not deducted by bookings here; bookings module owns subtraction
	Closed         bool     `json:"closed"`
	Rate           string   `json:"rate"`
	Currency       string   `json:"currency"`
	MinNights      *int     `json:"min_nights,omitempty"`
	MaxNights      *int     `json:"max_nights,omitempty"`
	ClosedToArrival   bool  `json:"closed_to_arrival,omitempty"`
	ClosedToDeparture bool  `json:"closed_to_departure,omitempty"`
}

// AvailabilityResponse is GET /availability's payload.
type AvailabilityResponse struct {
	HotelID uuid.UUID         `json:"hotel_id"`
	Start   Date              `json:"start"`
	End     Date              `json:"end"`
	Days    []AvailabilityDay `json:"days"`
}

// RuleListResponse is GET /pricing-rules' payload.
type RuleListResponse struct {
	Rules []PricingRule `json:"rules"`
}
