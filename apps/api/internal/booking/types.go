package booking

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Status is the booking lifecycle state.
type Status string

const (
	StatusPendingPayment Status = "pending_payment"
	StatusConfirmed      Status = "confirmed"
	StatusCancelled      Status = "cancelled"
	StatusExpired        Status = "expired"
	StatusCheckedIn      Status = "checked_in"
	StatusCheckedOut     Status = "checked_out"
	StatusNoShow         Status = "no_show"
	StatusCompleted      Status = "completed"
)

func (s Status) String() string { return string(s) }

// PaymentStatus tracks money state, independent of booking lifecycle.
type PaymentStatus string

const (
	PaymentPending  PaymentStatus = "pending"
	PaymentPaid     PaymentStatus = "paid"
	PaymentRefunded PaymentStatus = "refunded"
	PaymentPartial  PaymentStatus = "partial"
	PaymentFailed   PaymentStatus = "failed"
)

// Source identifies where the booking came from.
type Source string

const (
	SourceWeb    Source = "web"
	SourceWalkIn Source = "walk_in"
	SourcePhone  Source = "phone"
	SourceAdmin  Source = "admin"
)

type Booking struct {
	ID         uuid.UUID `json:"id"`
	Reference  string    `json:"reference"`
	HotelID    uuid.UUID `json:"hotel_id"`
	RoomTypeID uuid.UUID `json:"room_type_id"`
	RoomCount  int       `json:"room_count"`

	GuestEmail     string `json:"guest_email"`
	GuestPhone     string `json:"guest_phone,omitempty"`
	GuestName      string `json:"guest_name"`
	GuestCountry   string `json:"guest_country,omitempty"`
	SpecialRequest string `json:"special_request,omitempty"`

	CheckInDate  time.Time `json:"check_in_date"`
	CheckOutDate time.Time `json:"check_out_date"`
	Nights       int       `json:"nights"`

	Currency          string `json:"currency"`
	RoomSubtotalCents int64  `json:"room_subtotal_cents"`
	TaxesCents        int64  `json:"taxes_cents"`
	FeesCents         int64  `json:"fees_cents"`
	DiscountsCents    int64  `json:"discounts_cents"`
	TotalCents        int64  `json:"total_cents"`

	Status              Status        `json:"status"`
	PaymentStatus       PaymentStatus `json:"payment_status"`
	PaymentMethod       string        `json:"payment_method,omitempty"`
	ExpiresAt           *time.Time    `json:"expires_at,omitempty"`
	CancelledAt         *time.Time    `json:"cancelled_at,omitempty"`
	CancelledBy         string        `json:"cancelled_by,omitempty"`
	CancellationReason  string        `json:"cancellation_reason,omitempty"`

	Source      Source `json:"source"`
	UTMSource   string `json:"utm_source,omitempty"`
	UTMMedium   string `json:"utm_medium,omitempty"`
	UTMCampaign string `json:"utm_campaign,omitempty"`
	UTMTerm     string `json:"utm_term,omitempty"`
	UTMContent  string `json:"utm_content,omitempty"`
	Referrer    string `json:"referrer,omitempty"`

	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
	ConfirmedAt   *time.Time `json:"confirmed_at,omitempty"`
	CheckedInAt   *time.Time `json:"checked_in_at,omitempty"`
	CheckedOutAt  *time.Time `json:"checked_out_at,omitempty"`
}

// CreateRequest is the body used by both public (guest checkout) and admin
// (walk-in/phone) booking creation. Pricing breakdown is supplied by the
// caller — server re-validates against a server-side computation later
// (Phase 1 step). The booking row stores a snapshot regardless.
type CreateRequest struct {
	RoomTypeID uuid.UUID `json:"room_type_id"`
	RoomCount  int       `json:"room_count"`

	CheckInDate  string `json:"check_in_date"`  // YYYY-MM-DD
	CheckOutDate string `json:"check_out_date"` // YYYY-MM-DD

	GuestEmail     string `json:"guest_email"`
	GuestPhone     string `json:"guest_phone,omitempty"`
	GuestName      string `json:"guest_name"`
	GuestCountry   string `json:"guest_country,omitempty"`
	SpecialRequest string `json:"special_request,omitempty"`

	Currency          string `json:"currency,omitempty"`            // defaults to room_type base_currency
	RoomSubtotalCents int64  `json:"room_subtotal_cents,omitempty"` // defaults to base_rate * nights * rooms
	TaxesCents        int64  `json:"taxes_cents,omitempty"`
	FeesCents         int64  `json:"fees_cents,omitempty"`
	DiscountsCents    int64  `json:"discounts_cents,omitempty"`
	TotalCents        int64  `json:"total_cents,omitempty"`

	UTMSource   string `json:"utm_source,omitempty"`
	UTMMedium   string `json:"utm_medium,omitempty"`
	UTMCampaign string `json:"utm_campaign,omitempty"`
	UTMTerm     string `json:"utm_term,omitempty"`
	UTMContent  string `json:"utm_content,omitempty"`
	Referrer    string `json:"referrer,omitempty"`
}

type CancelRequest struct {
	Reason string `json:"reason,omitempty"`
}

type ListResponse struct {
	Bookings []Booking `json:"bookings"`
	Total    int       `json:"total"`
}

// PublicHotelContext is the slice of hotel metadata the guest UI needs
// alongside a booking confirmation — timezone for date formatting, currency
// for display, promptpay_id so the confirmation page can render a real
// EMVCo PromptPay QR. Intentionally duplicated from `landing.PublicHotelContext`
// rather than imported: keeps the booking package free of a hard dependency
// on the marketing/CMS module, and the type is tiny (5 string fields).
type PublicHotelContext struct {
	Name        string  `json:"name"`
	Slug        string  `json:"slug"`
	Timezone    string  `json:"timezone"`
	Currency    string  `json:"currency"`
	PromptPayID *string `json:"promptpay_id,omitempty"`
}

// PublicBookingResponse wraps a Booking with the hotel context the guest UI
// needs to format dates and render PromptPay QR codes on the confirmation
// page. Mirrors the shape of landing.PublicLandingResponse.
type PublicBookingResponse struct {
	Booking
	Hotel PublicHotelContext `json:"hotel"`
}

// BookingEvent is one row from the append-only booking_events audit table.
// Payload is opaque JSON whose shape depends on event_type.
type BookingEvent struct {
	ID        uuid.UUID       `json:"id"`
	BookingID uuid.UUID       `json:"booking_id"`
	EventType string          `json:"event_type"`
	ActorType string          `json:"actor_type,omitempty"`
	ActorID   *uuid.UUID      `json:"actor_id,omitempty"`
	Payload   json.RawMessage `json:"payload"`
	CreatedAt time.Time       `json:"created_at"`
}

// BookingEventsResponse is returned by
// GET /v1/hotels/{hotel_id}/bookings/{id}/events.
type BookingEventsResponse struct {
	Events []BookingEvent `json:"events"`
}
