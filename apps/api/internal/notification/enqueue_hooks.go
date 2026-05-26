package notification

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

// BookingInfo is the trimmed booking shape passed to enqueue hooks. We use a
// plain struct here rather than importing the booking package, which would
// create a circular dependency (booking → notification → booking).
//
// Callers build this from their own Booking row at the point of state change.
type BookingInfo struct {
	ID           uuid.UUID
	Reference    string
	HotelName    string
	GuestEmail   string
	GuestName    string
	CheckInDate  string // YYYY-MM-DD
	CheckOutDate string // YYYY-MM-DD
	Nights       int
	Currency     string
	TotalCents   int64
	Locale       string // "th" | "en"
}

// EnqueueBookingCreated fires when a booking transitions to pending_payment.
// It returns the persisted Notification or the error from Enqueue.
func (s *Service) EnqueueBookingCreated(ctx context.Context, b BookingInfo) (*Notification, error) {
	return s.Enqueue(ctx, ChannelEmail, TemplateBookingCreated, b.GuestEmail,
		bookingPayload(b, nil), "booking", &b.ID)
}

// EnqueueBookingConfirmed fires when payment is captured / hotel confirms.
func (s *Service) EnqueueBookingConfirmed(ctx context.Context, b BookingInfo) (*Notification, error) {
	return s.Enqueue(ctx, ChannelEmail, TemplateBookingConfirmed, b.GuestEmail,
		bookingPayload(b, nil), "booking", &b.ID)
}

// EnqueuePaymentClaimed fires to HOTEL staff (not guest) when a guest taps
// the "I have paid" button on the public confirmation page. hotelEmail is
// the recipient — the room's owner contact.
func (s *Service) EnqueuePaymentClaimed(ctx context.Context, b BookingInfo, hotelEmail string) (*Notification, error) {
	return s.Enqueue(ctx, ChannelEmail, TemplatePaymentClaimed, hotelEmail,
		bookingPayload(b, map[string]any{"guest_email": b.GuestEmail}),
		"booking", &b.ID)
}

// EnqueueBookingCancelled fires on cancellation. reason is optional.
func (s *Service) EnqueueBookingCancelled(ctx context.Context, b BookingInfo, reason string) (*Notification, error) {
	extra := map[string]any{}
	if reason != "" {
		extra["reason"] = reason
	}
	return s.Enqueue(ctx, ChannelEmail, TemplateBookingCancelled, b.GuestEmail,
		bookingPayload(b, extra), "booking", &b.ID)
}

// bookingPayload assembles the template variables. Money is formatted as a
// decimal string (we store cents in BIGINT) — currency is the hotel base.
func bookingPayload(b BookingInfo, extra map[string]any) map[string]any {
	locale := b.Locale
	if locale == "" {
		locale = "th"
	}
	p := map[string]any{
		"locale":         locale,
		"reference":      b.Reference,
		"hotel_name":     b.HotelName,
		"guest_name":     b.GuestName,
		"check_in_date":  b.CheckInDate,
		"check_out_date": b.CheckOutDate,
		"nights":         b.Nights,
		"currency":       b.Currency,
		"total":          formatMoneyCents(b.TotalCents),
	}
	for k, v := range extra {
		p[k] = v
	}
	return p
}

// formatMoneyCents turns 12345 → "123.45". Suitable for Phase 1 templates;
// the real i18n money formatter lives in the pricing module long-term.
func formatMoneyCents(c int64) string {
	neg := c < 0
	if neg {
		c = -c
	}
	whole := c / 100
	frac := c % 100
	s := fmt.Sprintf("%d.%02d", whole, frac)
	if neg {
		return "-" + s
	}
	return s
}
