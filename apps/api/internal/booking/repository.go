package booking

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// RoomTypeBasics is the minimum room-type info needed to price/validate a booking.
type RoomTypeBasics struct {
	HotelID        uuid.UUID
	HotelStatus    string
	HotelAccountID uuid.UUID
	TotalInventory int
	BaseRateCents  int64
	BaseCurrency   string
}

// GetRoomTypeBasics returns the parent hotel + base rate for a room_type.
// Returns ErrRoomTypeNotFound if the room_type doesn't exist or is soft-deleted.
func (r *Repository) GetRoomTypeBasics(ctx context.Context, roomTypeID uuid.UUID) (*RoomTypeBasics, error) {
	var rt RoomTypeBasics
	var rateNum float64
	err := r.db.QueryRow(ctx, `
		SELECT rt.hotel_id, h.status, h.account_id,
		       rt.total_inventory, rt.base_rate, rt.base_currency
		FROM room_types rt
		JOIN hotels h ON h.id = rt.hotel_id AND h.deleted_at IS NULL
		WHERE rt.id = $1 AND rt.deleted_at IS NULL
	`, roomTypeID).Scan(&rt.HotelID, &rt.HotelStatus, &rt.HotelAccountID,
		&rt.TotalInventory, &rateNum, &rt.BaseCurrency)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrRoomTypeNotFound
		}
		return nil, err
	}
	// base_rate stored as NUMERIC(10,2) — convert to cents.
	rt.BaseRateCents = int64(rateNum*100 + 0.5)
	return &rt, nil
}

// CreatePending creates a booking row with status=pending_payment under a
// pessimistic lock on the room_type row. Inside the same transaction we
// count active bookings overlapping each requested date and reject if any
// date would be oversold. This prevents the classic double-booking race.
func (r *Repository) CreatePending(
	ctx context.Context,
	b *Booking,
	holdDuration time.Duration,
	roomTypeID uuid.UUID,
) (*Booking, error) {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	// 1. Lock the room_type row to serialize concurrent booking attempts
	//    against the same inventory.
	var totalInventory int
	if err := tx.QueryRow(ctx, `
		SELECT total_inventory FROM room_types
		WHERE id = $1 AND deleted_at IS NULL
		FOR UPDATE
	`, roomTypeID).Scan(&totalInventory); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrRoomTypeNotFound
		}
		return nil, fmt.Errorf("lock room_type: %w", err)
	}

	// 2. Check availability for every night in the stay.
	if err := checkAvailability(ctx, tx, roomTypeID, totalInventory,
		b.CheckInDate, b.CheckOutDate, b.RoomCount); err != nil {
		return nil, err
	}

	// 3. Generate a unique reference (retry up to 5x on the unlikely collision).
	var reference string
	for attempt := 0; attempt < 5; attempt++ {
		ref, gerr := generateReference()
		if gerr != nil {
			return nil, fmt.Errorf("generate ref: %w", gerr)
		}
		var exists bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM bookings WHERE reference = $1)`, ref).Scan(&exists); err != nil {
			return nil, fmt.Errorf("check ref unique: %w", err)
		}
		if !exists {
			reference = ref
			break
		}
	}
	if reference == "" {
		return nil, errors.New("could not generate unique booking reference")
	}
	b.Reference = reference
	expiresAt := time.Now().Add(holdDuration)
	b.ExpiresAt = &expiresAt
	b.Status = StatusPendingPayment
	b.PaymentStatus = PaymentPending

	// 4. Insert.
	if err := scanBooking(tx.QueryRow(ctx, `
		INSERT INTO bookings (
			reference, hotel_id, room_type_id, room_count,
			guest_email, guest_phone, guest_name, guest_country, special_request,
			check_in_date, check_out_date,
			currency, room_subtotal_cents, taxes_cents, fees_cents, discounts_cents, total_cents,
			status, expires_at, payment_status,
			source, utm_source, utm_medium, utm_campaign, utm_term, utm_content, referrer
		)
		VALUES (
			$1, $2, $3, $4,
			$5, NULLIF($6,''), $7, NULLIF($8,''), NULLIF($9,''),
			$10, $11,
			$12, $13, $14, $15, $16, $17,
			$18, $19, $20,
			$21, NULLIF($22,''), NULLIF($23,''), NULLIF($24,''), NULLIF($25,''), NULLIF($26,''), NULLIF($27,'')
		)
		RETURNING `+bookingColumns,
		b.Reference, b.HotelID, b.RoomTypeID, b.RoomCount,
		b.GuestEmail, b.GuestPhone, b.GuestName, b.GuestCountry, b.SpecialRequest,
		b.CheckInDate, b.CheckOutDate,
		b.Currency, b.RoomSubtotalCents, b.TaxesCents, b.FeesCents, b.DiscountsCents, b.TotalCents,
		string(b.Status), *b.ExpiresAt, string(b.PaymentStatus),
		string(b.Source), b.UTMSource, b.UTMMedium, b.UTMCampaign, b.UTMTerm, b.UTMContent, b.Referrer,
	), b); err != nil {
		return nil, fmt.Errorf("insert booking: %w", err)
	}

	// 5. Audit event.
	if _, err := tx.Exec(ctx, `
		INSERT INTO booking_events (booking_id, event_type, actor_type, payload)
		VALUES ($1, 'created', $2, $3)
	`, b.ID, "system", mustJSON(map[string]any{"source": b.Source})); err != nil {
		return nil, fmt.Errorf("insert event: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}
	return b, nil
}

// checkAvailability verifies that for every night in [start, end), the total
// inventory (with any override) minus active bookings covers the requested
// room_count. Active bookings here = pending_payment | confirmed | checked_in.
func checkAvailability(
	ctx context.Context,
	tx pgx.Tx,
	roomTypeID uuid.UUID,
	totalInventory int,
	start, end time.Time,
	requested int,
) error {
	for d := start; d.Before(end); d = d.AddDate(0, 0, 1) {
		var inventoryChange int
		var closed bool
		err := tx.QueryRow(ctx, `
			SELECT inventory_change, closed
			FROM availability_overrides
			WHERE room_type_id = $1 AND date = $2
		`, roomTypeID, d).Scan(&inventoryChange, &closed)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("read override %s: %w", d.Format("2006-01-02"), err)
		}
		if closed {
			return ErrNoAvailability
		}
		var sold int
		if err := tx.QueryRow(ctx, `
			SELECT COALESCE(SUM(room_count), 0)
			FROM bookings
			WHERE room_type_id = $1
			  AND status IN ('pending_payment','confirmed','checked_in')
			  AND $2 >= check_in_date
			  AND $2 <  check_out_date
		`, roomTypeID, d).Scan(&sold); err != nil {
			return fmt.Errorf("count sold %s: %w", d.Format("2006-01-02"), err)
		}
		available := totalInventory + inventoryChange - sold
		if available < requested {
			return ErrNoAvailability
		}
	}
	return nil
}

// Confirm moves pending_payment → confirmed and zeroes expires_at.
func (r *Repository) Confirm(ctx context.Context, id uuid.UUID, actorType string, actorID *uuid.UUID) (*Booking, error) {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var b Booking
	if err := scanBooking(tx.QueryRow(ctx, `
		UPDATE bookings
		SET status = 'confirmed',
		    payment_status = 'paid',
		    confirmed_at = NOW(),
		    expires_at = NULL
		WHERE id = $1 AND status = 'pending_payment'
		RETURNING `+bookingColumns, id), &b); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrInvalidStateTransition
		}
		return nil, err
	}
	if err := appendEvent(ctx, tx, b.ID, "confirmed", actorType, actorID, nil); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &b, nil
}

// Cancel moves any active status (pending_payment/confirmed) to cancelled.
func (r *Repository) Cancel(ctx context.Context, id uuid.UUID, by, reason string, actorID *uuid.UUID) (*Booking, error) {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var b Booking
	if err := scanBooking(tx.QueryRow(ctx, `
		UPDATE bookings
		SET status = 'cancelled',
		    cancelled_at = NOW(),
		    cancelled_by = $2,
		    cancellation_reason = NULLIF($3, '')
		WHERE id = $1 AND status IN ('pending_payment','confirmed')
		RETURNING `+bookingColumns, id, by, reason), &b); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrInvalidStateTransition
		}
		return nil, err
	}
	if err := appendEvent(ctx, tx, b.ID, "cancelled", by, actorID,
		map[string]any{"reason": reason}); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &b, nil
}

// CheckIn moves confirmed → checked_in.
func (r *Repository) CheckIn(ctx context.Context, id uuid.UUID, actorID *uuid.UUID) (*Booking, error) {
	return r.transitionStaff(ctx, id, "confirmed", "checked_in", "checked_in_at", "checked_in", actorID)
}

// CheckOut moves checked_in → checked_out.
func (r *Repository) CheckOut(ctx context.Context, id uuid.UUID, actorID *uuid.UUID) (*Booking, error) {
	return r.transitionStaff(ctx, id, "checked_in", "checked_out", "checked_out_at", "checked_out", actorID)
}

// NoShow moves confirmed → no_show.
func (r *Repository) NoShow(ctx context.Context, id uuid.UUID, actorID *uuid.UUID) (*Booking, error) {
	return r.transitionStaff(ctx, id, "confirmed", "no_show", "", "no_show", actorID)
}

func (r *Repository) transitionStaff(
	ctx context.Context, id uuid.UUID,
	fromStatus, toStatus, timestampCol, eventType string,
	actorID *uuid.UUID,
) (*Booking, error) {
	tsClause := ""
	if timestampCol != "" {
		tsClause = ", " + timestampCol + " = NOW()"
	}
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var b Booking
	if err := scanBooking(tx.QueryRow(ctx, `
		UPDATE bookings SET status = $2`+tsClause+`
		WHERE id = $1 AND status = $3
		RETURNING `+bookingColumns, id, toStatus, fromStatus), &b); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrInvalidStateTransition
		}
		return nil, err
	}
	if err := appendEvent(ctx, tx, b.ID, eventType, "hotel_staff", actorID, nil); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &b, nil
}

// ExpirePending sweeps pending_payment rows whose expires_at has passed.
// Returns number of rows expired. Intended for the worker's periodic tick.
func (r *Repository) ExpirePending(ctx context.Context, now time.Time) (int64, error) {
	tag, err := r.db.Exec(ctx, `
		UPDATE bookings SET status = 'expired'
		WHERE status = 'pending_payment' AND expires_at < $1
	`, now)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// GetForAccount retrieves a booking by id, scoped to the caller's account
// (joins through hotels). Returns ErrBookingNotFound on mismatch.
func (r *Repository) GetForAccount(ctx context.Context, accountID, id uuid.UUID) (*Booking, error) {
	var b Booking
	err := scanBooking(r.db.QueryRow(ctx, `
		SELECT `+bookingColumns+`
		FROM bookings b
		JOIN hotels h ON h.id = b.hotel_id AND h.account_id = $2
		WHERE b.id = $1
	`, id, accountID), &b)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrBookingNotFound
		}
		return nil, err
	}
	return &b, nil
}

// GetByReference looks up a booking by its public reference (for guest lookup
// via /v1/public/bookings/{reference}). Caller must additionally verify the
// guest's email — that check lives in Service.
func (r *Repository) GetByReference(ctx context.Context, reference string) (*Booking, error) {
	var b Booking
	err := scanBooking(r.db.QueryRow(ctx, `
		SELECT `+bookingColumns+`
		FROM bookings
		WHERE reference = $1
	`, reference), &b)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrBookingNotFound
		}
		return nil, err
	}
	return &b, nil
}

// ListByHotel returns bookings for a hotel within the caller's account.
func (r *Repository) ListByHotel(ctx context.Context, accountID, hotelID uuid.UUID, status string, limit int) ([]Booking, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := r.db.Query(ctx, `
		SELECT `+bookingColumns+`
		FROM bookings b
		JOIN hotels h ON h.id = b.hotel_id AND h.account_id = $1
		WHERE b.hotel_id = $2
		  AND ($3 = '' OR b.status = $3)
		ORDER BY b.created_at DESC
		LIMIT $4
	`, accountID, hotelID, status, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []Booking{}
	for rows.Next() {
		var b Booking
		if err := scanBooking(rows, &b); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// ----- helpers -----

const bookingColumns = `
	id, reference, hotel_id, room_type_id, room_count,
	guest_email, COALESCE(guest_phone,''), guest_name, COALESCE(guest_country,''), COALESCE(special_request,''),
	check_in_date, check_out_date, nights,
	currency, room_subtotal_cents, taxes_cents, fees_cents, discounts_cents, total_cents,
	status, payment_status, COALESCE(payment_method,''),
	expires_at, cancelled_at, COALESCE(cancelled_by,''), COALESCE(cancellation_reason,''),
	source,
	COALESCE(utm_source,''), COALESCE(utm_medium,''), COALESCE(utm_campaign,''),
	COALESCE(utm_term,''), COALESCE(utm_content,''), COALESCE(referrer,''),
	created_at, updated_at, confirmed_at, checked_in_at, checked_out_at
`

// scanBooking scans a pgx.Row into b. Status/PaymentStatus/Source are stored
// as strings in Postgres; cast them back into the typed fields after Scan.
func scanBooking(row pgx.Row, b *Booking) error {
	var status, paymentStatus, source string
	if err := row.Scan(
		&b.ID, &b.Reference, &b.HotelID, &b.RoomTypeID, &b.RoomCount,
		&b.GuestEmail, &b.GuestPhone, &b.GuestName, &b.GuestCountry, &b.SpecialRequest,
		&b.CheckInDate, &b.CheckOutDate, &b.Nights,
		&b.Currency, &b.RoomSubtotalCents, &b.TaxesCents, &b.FeesCents, &b.DiscountsCents, &b.TotalCents,
		&status, &paymentStatus, &b.PaymentMethod,
		&b.ExpiresAt, &b.CancelledAt, &b.CancelledBy, &b.CancellationReason,
		&source,
		&b.UTMSource, &b.UTMMedium, &b.UTMCampaign, &b.UTMTerm, &b.UTMContent, &b.Referrer,
		&b.CreatedAt, &b.UpdatedAt, &b.ConfirmedAt, &b.CheckedInAt, &b.CheckedOutAt,
	); err != nil {
		return err
	}
	b.Status = Status(status)
	b.PaymentStatus = PaymentStatus(paymentStatus)
	b.Source = Source(source)
	return nil
}

func appendEvent(ctx context.Context, tx pgx.Tx, bookingID uuid.UUID, eventType, actorType string, actorID *uuid.UUID, payload map[string]any) error {
	var payloadJSON []byte
	if payload != nil {
		payloadJSON = mustJSON(payload)
	} else {
		payloadJSON = []byte(`{}`)
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO booking_events (booking_id, event_type, actor_type, actor_id, payload)
		VALUES ($1, $2, NULLIF($3,''), $4, $5)
	`, bookingID, eventType, actorType, actorID, payloadJSON)
	return err
}

func mustJSON(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}
