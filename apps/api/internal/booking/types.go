package booking

import (
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
