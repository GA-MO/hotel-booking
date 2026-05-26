package booking

import (
	"context"
	"log/slog"
	"net/mail"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/GA-MO/hotel-booking/apps/api/internal/platform/datetypes"
)

const defaultHold = 10 * time.Minute

// Event identifies a transition that may trigger external side effects
// (notifications, analytics, webhooks). Hooks are best-effort: an error
// from the hook is logged but does not unwind the booking change.
type Event string

const (
	EventCreated   Event = "created"
	EventConfirmed Event = "confirmed"
	EventCancelled Event = "cancelled"
	EventCheckedIn Event = "checked_in"
	EventCheckedOut Event = "checked_out"
	EventNoShow    Event = "no_show"
	// EventPaymentClaimed fires when the guest taps "I have paid" on the
	// public confirmation page. It is informational — the booking row's
	// payment_status does NOT change here; the hotel still has to verify
	// in their bank app and click Confirm.
	EventPaymentClaimed Event = "payment_claimed"
)

// EventHook is invoked after a successful state transition. The booking row
// passed in reflects the post-transition state. Implementations should do
// their work quickly — they share the HTTP request's context.
type EventHook func(ctx context.Context, event Event, b *Booking)

type Service struct {
	repo *Repository
	hold time.Duration
	hook EventHook
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo, hold: defaultHold}
}

// SetEventHook attaches an after-transition callback. Pass nil to clear.
// Returns the service so it can be chained at construction.
func (s *Service) SetEventHook(h EventHook) *Service {
	s.hook = h
	return s
}

// fire is a tiny wrapper that swallows panics from a misbehaving hook so they
// never affect the caller's HTTP response.
func (s *Service) fire(ctx context.Context, event Event, b *Booking) {
	if s.hook == nil || b == nil {
		return
	}
	defer func() {
		if r := recover(); r != nil {
			slog.Error("booking event hook panic", "event", event, "panic", r)
		}
	}()
	s.hook(ctx, event, b)
}

// CreatePublic builds a booking from a guest checkout (no auth). The hotel
// must be live (status='live') for public bookings to succeed.
func (s *Service) CreatePublic(ctx context.Context, hotelID uuid.UUID, req CreateRequest) (*Booking, error) {
	return s.create(ctx, hotelID, req, SourceWeb, true)
}

// CreateInternal is used by hotel staff for walk-in / phone bookings — the
// liveness check is skipped (staff can record bookings on test-mode hotels).
func (s *Service) CreateInternal(ctx context.Context, hotelID uuid.UUID, req CreateRequest, source Source) (*Booking, error) {
	return s.create(ctx, hotelID, req, source, false)
}

func (s *Service) create(
	ctx context.Context,
	hotelID uuid.UUID,
	req CreateRequest,
	source Source,
	requireLive bool,
) (*Booking, error) {
	// Validate guest.
	email, err := normalizeEmail(req.GuestEmail)
	if err != nil {
		return nil, ErrInvalidGuest
	}
	name := strings.TrimSpace(req.GuestName)
	if name == "" {
		return nil, ErrInvalidGuest
	}
	if req.RoomCount <= 0 {
		return nil, ErrInvalidRoomCount
	}

	checkIn, checkOut, err := parseDateRange(req.CheckInDate, req.CheckOutDate)
	if err != nil {
		return nil, err
	}

	// Resolve room_type → hotel + base rate. Verify it matches hotel_id and
	// the hotel is live (when required).
	rt, err := s.repo.GetRoomTypeBasics(ctx, req.RoomTypeID)
	if err != nil {
		return nil, err
	}
	if rt.HotelID != hotelID {
		return nil, ErrRoomTypeNotFound
	}
	if requireLive && rt.HotelStatus != "live" {
		return nil, ErrHotelNotLive
	}

	// Pricing: snapshot from request if supplied, otherwise compute the
	// simple base-rate × nights × rooms fallback. The pricing engine will
	// supersede this once the pricing module is wired in.
	currency := strings.ToUpper(strings.TrimSpace(req.Currency))
	if currency == "" {
		currency = rt.BaseCurrency
	}
	nights := int(checkOut.Sub(checkIn).Hours() / 24)
	subtotal := req.RoomSubtotalCents
	if subtotal == 0 {
		subtotal = rt.BaseRateCents * int64(nights) * int64(req.RoomCount)
	}
	total := req.TotalCents
	if total == 0 {
		total = subtotal + req.TaxesCents + req.FeesCents - req.DiscountsCents
	}

	b := &Booking{
		HotelID:           hotelID,
		RoomTypeID:        req.RoomTypeID,
		RoomCount:         req.RoomCount,
		GuestEmail:        email,
		GuestPhone:        strings.TrimSpace(req.GuestPhone),
		GuestName:         name,
		GuestCountry:      strings.ToUpper(strings.TrimSpace(req.GuestCountry)),
		SpecialRequest:    strings.TrimSpace(req.SpecialRequest),
		CheckInDate:       datetypes.Date(checkIn),
		CheckOutDate:      datetypes.Date(checkOut),
		Nights:            nights,
		Currency:          currency,
		RoomSubtotalCents: subtotal,
		TaxesCents:        req.TaxesCents,
		FeesCents:         req.FeesCents,
		DiscountsCents:    req.DiscountsCents,
		TotalCents:        total,
		Source:            source,
		UTMSource:         req.UTMSource,
		UTMMedium:         req.UTMMedium,
		UTMCampaign:       req.UTMCampaign,
		UTMTerm:           req.UTMTerm,
		UTMContent:        req.UTMContent,
		Referrer:          req.Referrer,
	}
	out, err := s.repo.CreatePending(ctx, b, s.hold, req.RoomTypeID)
	if err == nil {
		s.fire(ctx, EventCreated, out)
	}
	return out, err
}

// Confirm a pending booking (after payment).
func (s *Service) Confirm(ctx context.Context, id uuid.UUID, actorType string, actorID *uuid.UUID) (*Booking, error) {
	out, err := s.repo.Confirm(ctx, id, actorType, actorID)
	if err == nil {
		s.fire(ctx, EventConfirmed, out)
	}
	return out, err
}

// CancelByHotel — staff-initiated cancellation.
func (s *Service) CancelByHotel(ctx context.Context, id uuid.UUID, reason string, actorID uuid.UUID) (*Booking, error) {
	out, err := s.repo.Cancel(ctx, id, "hotel", reason, &actorID)
	if err == nil {
		s.fire(ctx, EventCancelled, out)
	}
	return out, err
}

// CancelByGuest — guest-initiated cancellation via reference + email verify.
// Returns the cancelled booking wrapped with the hotel context so the
// confirmation/cancellation UI doesn't need a second round-trip.
func (s *Service) CancelByGuest(ctx context.Context, reference, email, reason string) (*PublicBookingResponse, error) {
	b, hctx, err := s.repo.GetByReferenceWithHotel(ctx, reference)
	if err != nil {
		return nil, err
	}
	if !strings.EqualFold(b.GuestEmail, strings.TrimSpace(email)) {
		return nil, ErrBookingNotFound
	}
	out, err := s.repo.Cancel(ctx, b.ID, "guest", reason, nil)
	if err != nil {
		return nil, err
	}
	s.fire(ctx, EventCancelled, out)
	return &PublicBookingResponse{Booking: *out, Hotel: *hctx}, nil
}

// MarkPaymentClaimed is fired by the guest tapping "I have paid" on the
// public confirmation page. It does NOT change booking state — the hotel
// still needs to verify and Confirm. We append a booking_event so the admin
// timeline reflects the claim, and fire the EventPaymentClaimed hook so a
// notification can be enqueued to staff.
func (s *Service) MarkPaymentClaimed(ctx context.Context, reference, email string) (*PublicBookingResponse, error) {
	b, hctx, err := s.repo.GetByReferenceWithHotel(ctx, reference)
	if err != nil {
		return nil, err
	}
	if !strings.EqualFold(b.GuestEmail, strings.TrimSpace(email)) {
		return nil, ErrBookingNotFound
	}
	if b.Status != StatusPendingPayment {
		// Only meaningful while we're holding the reservation; once confirmed
		// the hotel has already verified payment.
		return nil, ErrInvalidStateTransition
	}
	if err := s.repo.AppendEvent(ctx, b.ID, string(EventPaymentClaimed), "guest", nil, nil); err != nil {
		return nil, err
	}
	s.fire(ctx, EventPaymentClaimed, b)
	return &PublicBookingResponse{Booking: *b, Hotel: *hctx}, nil
}

func (s *Service) CheckIn(ctx context.Context, id uuid.UUID, actorID uuid.UUID) (*Booking, error) {
	out, err := s.repo.CheckIn(ctx, id, &actorID)
	if err == nil {
		s.fire(ctx, EventCheckedIn, out)
	}
	return out, err
}
func (s *Service) CheckOut(ctx context.Context, id uuid.UUID, actorID uuid.UUID) (*Booking, error) {
	out, err := s.repo.CheckOut(ctx, id, &actorID)
	if err == nil {
		s.fire(ctx, EventCheckedOut, out)
	}
	return out, err
}
func (s *Service) NoShow(ctx context.Context, id uuid.UUID, actorID uuid.UUID) (*Booking, error) {
	out, err := s.repo.NoShow(ctx, id, &actorID)
	if err == nil {
		s.fire(ctx, EventNoShow, out)
	}
	return out, err
}

// GetForAccount fetches a booking scoped to the caller's account.
func (s *Service) GetForAccount(ctx context.Context, accountID, id uuid.UUID) (*Booking, error) {
	return s.repo.GetForAccount(ctx, accountID, id)
}

// GetPublic looks up by reference + email — for guest self-service. Returns
// the booking wrapped with the hotel context (timezone / currency /
// promptpay_id) so the confirmation page can render dates and a PromptPay
// QR without a second round-trip.
func (s *Service) GetPublic(ctx context.Context, reference, email string) (*PublicBookingResponse, error) {
	b, hctx, err := s.repo.GetByReferenceWithHotel(ctx, reference)
	if err != nil {
		return nil, err
	}
	if !strings.EqualFold(b.GuestEmail, strings.TrimSpace(email)) {
		return nil, ErrBookingNotFound
	}
	return &PublicBookingResponse{Booking: *b, Hotel: *hctx}, nil
}

// ListEvents returns the audit trail for one booking, scoped to the caller's
// account + hotel. Cross-tenant access returns ErrBookingNotFound.
func (s *Service) ListEvents(ctx context.Context, accountID, hotelID, bookingID uuid.UUID) ([]BookingEvent, error) {
	return s.repo.ListEvents(ctx, accountID, hotelID, bookingID)
}

// ListByHotel returns bookings for a hotel, scoped to the caller's account.
// Pass status="" to include all.
func (s *Service) ListByHotel(ctx context.Context, accountID, hotelID uuid.UUID, status string, limit int) ([]Booking, error) {
	return s.repo.ListByHotel(ctx, accountID, hotelID, status, limit)
}

// ExpirePending is called periodically by the worker.
func (s *Service) ExpirePending(ctx context.Context) (int64, error) {
	return s.repo.ExpirePending(ctx, time.Now())
}

// ----- validation helpers -----

func normalizeEmail(in string) (string, error) {
	in = strings.TrimSpace(strings.ToLower(in))
	addr, err := mail.ParseAddress(in)
	if err != nil {
		return "", ErrInvalidGuest
	}
	return addr.Address, nil
}

func parseDateRange(in, out string) (time.Time, time.Time, error) {
	const layout = "2006-01-02"
	ci, err := time.Parse(layout, strings.TrimSpace(in))
	if err != nil {
		return time.Time{}, time.Time{}, ErrInvalidDates
	}
	co, err := time.Parse(layout, strings.TrimSpace(out))
	if err != nil {
		return time.Time{}, time.Time{}, ErrInvalidDates
	}
	if !co.After(ci) {
		return time.Time{}, time.Time{}, ErrInvalidDates
	}
	return ci, co, nil
}
