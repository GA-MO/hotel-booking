package pricing

import (
	"context"
	"errors"
	"fmt"
	"strings"
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

// hotelOwnedByAccount returns true if the hotel exists, is not soft-deleted,
// and belongs to the caller's account. We treat cross-tenant access as
// not-found to avoid leaking existence (mirrors the roomtype repo).
func (r *Repository) hotelOwnedByAccount(ctx context.Context, accountID, hotelID uuid.UUID) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM hotels
			WHERE id = $1 AND account_id = $2 AND deleted_at IS NULL
		)
	`, hotelID, accountID).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}

// roomTypeOwnedByHotel reports whether the given room_type exists, is not
// deleted, and belongs to the given hotel.
func (r *Repository) roomTypeOwnedByHotel(ctx context.Context, hotelID, roomTypeID uuid.UUID) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM room_types
			WHERE id = $1 AND hotel_id = $2 AND deleted_at IS NULL
		)
	`, roomTypeID, hotelID).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}

// HotelBaseInfo is the minimal hotel context the engine needs to quote a stay.
type HotelBaseInfo struct {
	ID           uuid.UUID
	Slug         string
	Status       string
	Timezone     string
	BaseCurrency string
}

// LookupHotelBySlug returns the hotel by slug if it is live (used by the
// public quote endpoint).
func (r *Repository) LookupHotelBySlug(ctx context.Context, slug string) (*HotelBaseInfo, error) {
	var h HotelBaseInfo
	err := r.db.QueryRow(ctx, `
		SELECT id, slug, status, timezone, base_currency
		FROM hotels
		WHERE slug = $1 AND deleted_at IS NULL
	`, slug).Scan(&h.ID, &h.Slug, &h.Status, &h.Timezone, &h.BaseCurrency)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrHotelNotFound
		}
		return nil, err
	}
	if h.Status != "live" {
		return nil, ErrHotelNotFound
	}
	return &h, nil
}

// RoomTypeBaseInfo is the minimal room_type context the engine needs.
type RoomTypeBaseInfo struct {
	ID             uuid.UUID
	HotelID        uuid.UUID
	TotalInventory int
	BaseRateMinor  int64
	BaseCurrency   string
}

// GetRoomTypeForQuote returns the lean room_type fields the engine needs,
// scoped to the given hotel (and ensures it isn't soft-deleted or disabled).
// This skips the account check: it's used by the public quote endpoint where
// we've already resolved the hotel from the slug.
func (r *Repository) GetRoomTypeForQuote(ctx context.Context, hotelID, roomTypeID uuid.UUID) (*RoomTypeBaseInfo, error) {
	var (
		rt       RoomTypeBaseInfo
		baseRate string
	)
	err := r.db.QueryRow(ctx, `
		SELECT id, hotel_id, total_inventory, base_rate::text, base_currency
		FROM room_types
		WHERE id = $1 AND hotel_id = $2
		  AND deleted_at IS NULL AND enabled = TRUE
	`, roomTypeID, hotelID).Scan(&rt.ID, &rt.HotelID, &rt.TotalInventory, &baseRate, &rt.BaseCurrency)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrRoomTypeNotFound
		}
		return nil, err
	}
	rt.BaseRateMinor, err = parseMinor(baseRate)
	if err != nil {
		return nil, fmt.Errorf("parse base_rate %q: %w", baseRate, err)
	}
	return &rt, nil
}

// ListRoomTypesForHotel returns lean info for every enabled, non-deleted
// room_type under a hotel — used for the GET /availability endpoint.
func (r *Repository) ListRoomTypesForHotel(ctx context.Context, hotelID uuid.UUID) ([]RoomTypeBaseInfo, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, hotel_id, total_inventory, base_rate::text, base_currency
		FROM room_types
		WHERE hotel_id = $1 AND deleted_at IS NULL AND enabled = TRUE
		ORDER BY display_order ASC, created_at ASC
	`, hotelID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []RoomTypeBaseInfo{}
	for rows.Next() {
		var (
			rt       RoomTypeBaseInfo
			baseRate string
		)
		if err := rows.Scan(&rt.ID, &rt.HotelID, &rt.TotalInventory, &baseRate, &rt.BaseCurrency); err != nil {
			return nil, err
		}
		rt.BaseRateMinor, err = parseMinor(baseRate)
		if err != nil {
			return nil, fmt.Errorf("parse base_rate %q: %w", baseRate, err)
		}
		result = append(result, rt)
	}
	return result, rows.Err()
}

// MinAvailableForRange returns the minimum number of rooms still available
// across every night in [start, end), accounting for inventory overrides and
// active bookings (pending_payment | confirmed | checked_in). The boolean
// reports whether any day in the range is hotel-closed — when true, the
// caller should treat availability as 0 regardless of inventory math.
//
// Mirrors the SQL in booking.checkAvailability, but returns the count instead
// of a sentinel error so the public quote endpoint can surface "X rooms left"
// to the guest before they commit. No row lock here — this is a read-only
// preview; commitment still goes through the booking module's FOR UPDATE
// path.
func (r *Repository) MinAvailableForRange(
	ctx context.Context,
	roomTypeID uuid.UUID,
	totalInventory int,
	start, end time.Time,
) (available int, anyClosed bool, err error) {
	var minAvail *int
	err = r.db.QueryRow(ctx, `
		WITH days AS (
			SELECT generate_series($2::date, ($3::date - 1), '1 day'::interval)::date AS d
		)
		SELECT
			COALESCE(bool_or(COALESCE(ov.closed, false)), false) AS any_closed,
			MIN($4::int + COALESCE(ov.inventory_change, 0) - COALESCE(s.sold, 0)) AS min_available
		FROM days
		LEFT JOIN availability_overrides ov
			ON ov.room_type_id = $1 AND ov.date = days.d
		LEFT JOIN LATERAL (
			SELECT COALESCE(SUM(b.room_count), 0)::int AS sold
			FROM bookings b
			WHERE b.room_type_id = $1
			  AND b.status IN ('pending_payment','confirmed','checked_in')
			  AND days.d >= b.check_in_date
			  AND days.d <  b.check_out_date
		) s ON true
	`, roomTypeID, start, end, totalInventory).Scan(&anyClosed, &minAvail)
	if err != nil {
		return 0, false, fmt.Errorf("min available: %w", err)
	}
	if anyClosed {
		return 0, true, nil
	}
	// minAvail is NULL only when the date range produced no rows. The service
	// layer guards against that with validateDateRange, but if we ever reach
	// here treat it as zero rather than panic on the deref.
	if minAvail == nil {
		return 0, false, nil
	}
	if *minAvail < 0 {
		return 0, false, nil
	}
	return *minAvail, false, nil
}

// ----- availability_overrides -----

const overrideColumns = `
	hotel_id, room_type_id, date,
	inventory_change, closed,
	rate_override::text, rate_currency,
	min_nights, max_nights,
	closed_to_arrival, closed_to_departure,
	created_at, updated_at
`

func scanOverride(row pgx.Row) (*AvailabilityOverride, error) {
	var (
		ov           AvailabilityOverride
		date         time.Time
		rateOverride *string
	)
	if err := row.Scan(
		&ov.HotelID, &ov.RoomTypeID, &date,
		&ov.InventoryChange, &ov.Closed,
		&rateOverride, &ov.RateCurrency,
		&ov.MinNights, &ov.MaxNights,
		&ov.ClosedToArrival, &ov.ClosedToDeparture,
		&ov.CreatedAt, &ov.UpdatedAt,
	); err != nil {
		return nil, err
	}
	ov.Date = Date{date}
	if rateOverride != nil {
		minor, err := parseMinor(*rateOverride)
		if err != nil {
			return nil, fmt.Errorf("parse rate_override %q: %w", *rateOverride, err)
		}
		ov.RateOverrideMinor = &minor
		s := *rateOverride
		// Normalise the wire format (DB returns "1500.00" already, but be defensive).
		ov.RateOverride = &s
	}
	return &ov, nil
}

// ListOverridesInRange returns all overrides for a hotel between [start, end)
// (the end date is exclusive — matches the booking convention).
func (r *Repository) ListOverridesInRange(ctx context.Context, accountID, hotelID uuid.UUID, start, end time.Time) ([]AvailabilityOverride, error) {
	ok, err := r.hotelOwnedByAccount(ctx, accountID, hotelID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrHotelNotFound
	}
	return r.listOverridesNoAuth(ctx, hotelID, start, end, nil)
}

// listOverridesNoAuth skips the account scope check — used by the public quote
// endpoint where the hotel was already resolved by slug.
func (r *Repository) listOverridesNoAuth(ctx context.Context, hotelID uuid.UUID, start, end time.Time, roomTypeID *uuid.UUID) ([]AvailabilityOverride, error) {
	q := `SELECT ` + overrideColumns + `
		FROM availability_overrides
		WHERE hotel_id = $1 AND date >= $2 AND date < $3`
	args := []any{hotelID, start, end}
	if roomTypeID != nil {
		q += ` AND room_type_id = $4`
		args = append(args, *roomTypeID)
	}
	q += ` ORDER BY room_type_id, date`
	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []AvailabilityOverride{}
	for rows.Next() {
		ov, err := scanOverride(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, *ov)
	}
	return result, rows.Err()
}

// UpsertOverrides atomically inserts/updates a batch of override rows.
// Every room_type_id in items must belong to the hotel.
func (r *Repository) UpsertOverrides(ctx context.Context, accountID, hotelID uuid.UUID, items []UpsertAvailabilityItem) error {
	ok, err := r.hotelOwnedByAccount(ctx, accountID, hotelID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrHotelNotFound
	}
	if len(items) == 0 {
		return nil
	}

	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Verify every room_type belongs to this hotel.
	rtSet := make(map[uuid.UUID]struct{}, len(items))
	for _, it := range items {
		rtSet[it.RoomTypeID] = struct{}{}
	}
	rtIDs := make([]uuid.UUID, 0, len(rtSet))
	for id := range rtSet {
		rtIDs = append(rtIDs, id)
	}
	var found int
	if err := tx.QueryRow(ctx, `
		SELECT COUNT(*) FROM room_types
		WHERE hotel_id = $1 AND deleted_at IS NULL AND id = ANY($2::uuid[])
	`, hotelID, rtIDs).Scan(&found); err != nil {
		return err
	}
	if found != len(rtSet) {
		return ErrRoomTypeNotFound
	}

	const upsert = `
		INSERT INTO availability_overrides (
			hotel_id, room_type_id, date,
			inventory_change, closed,
			rate_override, rate_currency,
			min_nights, max_nights,
			closed_to_arrival, closed_to_departure
		)
		VALUES ($1, $2, $3, $4, $5,
		        NULLIF($6, '')::numeric, NULLIF($7, ''),
		        $8, $9, $10, $11)
		ON CONFLICT (hotel_id, room_type_id, date) DO UPDATE SET
		  inventory_change   = EXCLUDED.inventory_change,
		  closed             = EXCLUDED.closed,
		  rate_override      = EXCLUDED.rate_override,
		  rate_currency      = EXCLUDED.rate_currency,
		  min_nights         = EXCLUDED.min_nights,
		  max_nights         = EXCLUDED.max_nights,
		  closed_to_arrival  = EXCLUDED.closed_to_arrival,
		  closed_to_departure= EXCLUDED.closed_to_departure
	`
	for _, it := range items {
		rateOverride := ""
		if it.RateOverride != nil {
			rateOverride = strings.TrimSpace(*it.RateOverride)
		}
		rateCurrency := ""
		if it.RateCurrency != nil {
			rateCurrency = strings.TrimSpace(*it.RateCurrency)
		}
		_, err := tx.Exec(ctx, upsert,
			hotelID, it.RoomTypeID, it.Date.Time,
			it.InventoryChange, it.Closed,
			rateOverride, rateCurrency,
			it.MinNights, it.MaxNights,
			it.ClosedToArrival, it.ClosedToDeparture,
		)
		if err != nil {
			return fmt.Errorf("upsert availability_override: %w", err)
		}
	}
	return tx.Commit(ctx)
}

// DeleteOverride removes a single override row.
func (r *Repository) DeleteOverride(ctx context.Context, accountID, hotelID, roomTypeID uuid.UUID, date time.Time) error {
	ok, err := r.hotelOwnedByAccount(ctx, accountID, hotelID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrHotelNotFound
	}
	tag, err := r.db.Exec(ctx, `
		DELETE FROM availability_overrides
		WHERE hotel_id = $1 AND room_type_id = $2 AND date = $3
	`, hotelID, roomTypeID, date)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrRoomTypeNotFound
	}
	return nil
}

// ----- pricing_rules -----

const ruleColumns = `
	id, hotel_id, room_type_id,
	name, rule_type,
	start_date, end_date, days_of_week,
	modifier_type, modifier_value::text,
	min_nights, max_nights, min_days_ahead, max_days_ahead,
	priority, enabled,
	created_at, updated_at
`

func scanRule(row pgx.Row) (*PricingRule, error) {
	var (
		pr            PricingRule
		startDate     *time.Time
		endDate       *time.Time
		modifierValue string
	)
	if err := row.Scan(
		&pr.ID, &pr.HotelID, &pr.RoomTypeID,
		&pr.Name, &pr.RuleType,
		&startDate, &endDate, &pr.DaysOfWeek,
		&pr.ModifierType, &modifierValue,
		&pr.MinNights, &pr.MaxNights, &pr.MinDaysAhead, &pr.MaxDaysAhead,
		&pr.Priority, &pr.Enabled,
		&pr.CreatedAt, &pr.UpdatedAt,
	); err != nil {
		return nil, err
	}
	if startDate != nil {
		pr.StartDate = &Date{*startDate}
	}
	if endDate != nil {
		pr.EndDate = &Date{*endDate}
	}
	pr.ModifierValue = modifierValue
	return &pr, nil
}

// ListRules returns every rule for a hotel (enabled and disabled), sorted
// deterministically.
func (r *Repository) ListRules(ctx context.Context, accountID, hotelID uuid.UUID) ([]PricingRule, error) {
	ok, err := r.hotelOwnedByAccount(ctx, accountID, hotelID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrHotelNotFound
	}
	return r.listRulesNoAuth(ctx, hotelID)
}

// listRulesNoAuth — used by public quote after resolving hotel via slug.
func (r *Repository) listRulesNoAuth(ctx context.Context, hotelID uuid.UUID) ([]PricingRule, error) {
	rows, err := r.db.Query(ctx, `
		SELECT `+ruleColumns+`
		FROM pricing_rules
		WHERE hotel_id = $1
		ORDER BY priority ASC, created_at ASC
	`, hotelID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []PricingRule{}
	for rows.Next() {
		pr, err := scanRule(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, *pr)
	}
	return result, rows.Err()
}

func (r *Repository) GetRule(ctx context.Context, accountID, hotelID, id uuid.UUID) (*PricingRule, error) {
	ok, err := r.hotelOwnedByAccount(ctx, accountID, hotelID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrHotelNotFound
	}
	row := r.db.QueryRow(ctx, `
		SELECT `+ruleColumns+`
		FROM pricing_rules
		WHERE id = $1 AND hotel_id = $2
	`, id, hotelID)
	pr, err := scanRule(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrRuleNotFound
		}
		return nil, err
	}
	return pr, nil
}

func (r *Repository) CreateRule(ctx context.Context, accountID, hotelID uuid.UUID, req CreateRuleRequest) (*PricingRule, error) {
	ok, err := r.hotelOwnedByAccount(ctx, accountID, hotelID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrHotelNotFound
	}
	if req.RoomTypeID != nil {
		owned, err := r.roomTypeOwnedByHotel(ctx, hotelID, *req.RoomTypeID)
		if err != nil {
			return nil, err
		}
		if !owned {
			return nil, ErrRoomTypeNotFound
		}
	}

	var startDate, endDate any
	if req.StartDate != nil {
		startDate = req.StartDate.Time
	}
	if req.EndDate != nil {
		endDate = req.EndDate.Time
	}
	priority := 100
	if req.Priority != nil {
		priority = *req.Priority
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	row := r.db.QueryRow(ctx, `
		INSERT INTO pricing_rules (
			hotel_id, room_type_id,
			name, rule_type,
			start_date, end_date, days_of_week,
			modifier_type, modifier_value,
			min_nights, max_nights, min_days_ahead, max_days_ahead,
			priority, enabled
		)
		VALUES ($1, $2,
		        $3, $4,
		        $5, $6, $7,
		        $8, $9::numeric,
		        $10, $11, $12, $13,
		        $14, $15)
		RETURNING `+ruleColumns+`
	`,
		hotelID, req.RoomTypeID,
		req.Name, req.RuleType,
		startDate, endDate, intSliceOrNil(req.DaysOfWeek),
		req.ModifierType, req.ModifierValue,
		req.MinNights, req.MaxNights, req.MinDaysAhead, req.MaxDaysAhead,
		priority, enabled,
	)
	pr, err := scanRule(row)
	if err != nil {
		return nil, fmt.Errorf("insert pricing_rule: %w", err)
	}
	return pr, nil
}

func (r *Repository) UpdateRule(ctx context.Context, accountID, hotelID, id uuid.UUID, patch UpdateRuleRequest) (*PricingRule, error) {
	ok, err := r.hotelOwnedByAccount(ctx, accountID, hotelID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrHotelNotFound
	}
	var startDate, endDate any
	if patch.StartDate != nil {
		startDate = patch.StartDate.Time
	}
	if patch.EndDate != nil {
		endDate = patch.EndDate.Time
	}
	var dow any
	if patch.DaysOfWeek != nil {
		dow = intSliceOrNil(*patch.DaysOfWeek)
	}
	tag, err := r.db.Exec(ctx, `
		UPDATE pricing_rules SET
		  name           = COALESCE($3, name),
		  start_date     = COALESCE($4::date,    start_date),
		  end_date       = COALESCE($5::date,    end_date),
		  days_of_week   = COALESCE($6::int[],   days_of_week),
		  modifier_type  = COALESCE($7,          modifier_type),
		  modifier_value = COALESCE($8::numeric, modifier_value),
		  min_nights     = COALESCE($9,  min_nights),
		  max_nights     = COALESCE($10, max_nights),
		  min_days_ahead = COALESCE($11, min_days_ahead),
		  max_days_ahead = COALESCE($12, max_days_ahead),
		  priority       = COALESCE($13, priority),
		  enabled        = COALESCE($14, enabled)
		WHERE id = $1 AND hotel_id = $2
	`,
		id, hotelID,
		ptrOrNil(patch.Name),
		startDate, endDate, dow,
		ptrOrNil(patch.ModifierType),
		ptrOrNil(patch.ModifierValue),
		ptrOrNil(patch.MinNights),
		ptrOrNil(patch.MaxNights),
		ptrOrNil(patch.MinDaysAhead),
		ptrOrNil(patch.MaxDaysAhead),
		ptrOrNil(patch.Priority),
		ptrOrNil(patch.Enabled),
	)
	if err != nil {
		return nil, fmt.Errorf("update pricing_rule: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return nil, ErrRuleNotFound
	}
	return r.GetRule(ctx, accountID, hotelID, id)
}

func (r *Repository) DeleteRule(ctx context.Context, accountID, hotelID, id uuid.UUID) error {
	ok, err := r.hotelOwnedByAccount(ctx, accountID, hotelID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrHotelNotFound
	}
	tag, err := r.db.Exec(ctx, `
		DELETE FROM pricing_rules WHERE id = $1 AND hotel_id = $2
	`, id, hotelID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrRuleNotFound
	}
	return nil
}

// ----- helpers -----

func ptrOrNil[T any](p *T) any {
	if p == nil {
		return nil
	}
	return *p
}

func intSliceOrNil(s []int) any {
	if s == nil {
		return nil
	}
	return s
}
