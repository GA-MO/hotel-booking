package pricing

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Service struct {
	repo *Repository
	now  func() time.Time // injectable for tests
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo, now: time.Now}
}

const (
	nameMaxLen        = 120
	maxRangeDays      = 366
	maxBatchOverrides = 366 * 32 // 1 year × 32 room types
	maxRooms          = 50
)

// ----- availability -----

// GetAvailability returns the per-day computed rate + inventory state for
// every enabled room_type in the hotel, across [start, end). The booking
// module is responsible for deducting existing reservations; this endpoint
// only exposes hotel-controlled inventory + closures.
func (s *Service) GetAvailability(ctx context.Context, accountID, hotelID uuid.UUID, start, end time.Time) (*AvailabilityResponse, error) {
	if err := validateDateRange(start, end, maxRangeDays); err != nil {
		return nil, err
	}
	if ok, err := s.repo.hotelOwnedByAccount(ctx, accountID, hotelID); err != nil {
		return nil, err
	} else if !ok {
		return nil, ErrHotelNotFound
	}

	roomTypes, err := s.repo.ListRoomTypesForHotel(ctx, hotelID)
	if err != nil {
		return nil, err
	}
	overrides, err := s.repo.listOverridesNoAuth(ctx, hotelID, start, end, nil)
	if err != nil {
		return nil, err
	}
	rules, err := s.repo.listRulesNoAuth(ctx, hotelID)
	if err != nil {
		return nil, err
	}

	// Index overrides for O(1) lookup.
	type ovKey struct {
		rt   uuid.UUID
		date string
	}
	idx := make(map[ovKey]AvailabilityOverride, len(overrides))
	for _, ov := range overrides {
		idx[ovKey{ov.RoomTypeID, ov.Date.Format(DateLayout)}] = ov
	}

	days := []AvailabilityDay{}
	today := s.startOfDay(s.now())

	for _, rt := range roomTypes {
		// Build engine rules scoped to this room_type (rules with NULL room_type
		// apply to all; rules with a specific room_type only to that one).
		scoped := make([]EngineRule, 0, len(rules))
		for _, pr := range rules {
			if !pr.Enabled {
				continue
			}
			if pr.RoomTypeID != nil && *pr.RoomTypeID != rt.ID {
				continue
			}
			er, err := ToEngineRule(pr)
			if err != nil {
				return nil, err
			}
			scoped = append(scoped, er)
		}

		// One-day quote per day so we surface the post-rules nightly rate.
		for cur := start; cur.Before(end); cur = cur.AddDate(0, 0, 1) {
			ovOnce := map[string]EngineOverride{}
			day := AvailabilityDay{
				Date:           Date{cur},
				RoomTypeID:     rt.ID,
				TotalInventory: rt.TotalInventory,
				Available:      rt.TotalInventory,
				Currency:       rt.BaseCurrency,
			}
			if ov, ok := idx[ovKey{rt.ID, cur.Format(DateLayout)}]; ok {
				day.Closed = ov.Closed
				day.Available = rt.TotalInventory + ov.InventoryChange
				if day.Available < 0 {
					day.Available = 0
				}
				day.MinNights = ov.MinNights
				day.MaxNights = ov.MaxNights
				day.ClosedToArrival = ov.ClosedToArrival
				day.ClosedToDeparture = ov.ClosedToDeparture
				if ov.RateOverrideMinor != nil {
					ovOnce[cur.Format(DateLayout)] = EngineOverride{
						RateOverrideMinor: ov.RateOverrideMinor,
						RateCurrency:      ov.RateCurrency,
					}
				}
			}

			next := cur.AddDate(0, 0, 1)
			q, err := CalculateQuote(QuoteInput{
				BaseRateMinor: rt.BaseRateMinor,
				Currency:      rt.BaseCurrency,
				Overrides:     ovOnce,
				Rules:         scoped,
				CheckIn:       cur,
				CheckOut:      next,
				Today:         today,
			})
			if err != nil {
				return nil, err
			}
			if len(q.PerNight) > 0 {
				day.Rate = q.PerNight[0].Amount
				day.Currency = q.PerNight[0].Currency
			} else {
				day.Rate = formatMinor(rt.BaseRateMinor)
			}
			days = append(days, day)
		}
	}

	return &AvailabilityResponse{
		HotelID: hotelID,
		Start:   Date{start},
		End:     Date{end},
		Days:    days,
	}, nil
}

// UpsertAvailability validates and writes a batch of override rows.
func (s *Service) UpsertAvailability(ctx context.Context, accountID, hotelID uuid.UUID, req UpsertAvailabilityRequest) error {
	if len(req.Items) == 0 {
		return ErrInvalidRequest
	}
	if len(req.Items) > maxBatchOverrides {
		return fmt.Errorf("%w: batch too large (max %d)", ErrInvalidRequest, maxBatchOverrides)
	}
	for i := range req.Items {
		it := &req.Items[i]
		if it.RoomTypeID == uuid.Nil {
			return ErrInvalidRequest
		}
		if it.Date.IsZero() {
			return ErrInvalidDate
		}
		if it.MinNights != nil && *it.MinNights < 1 {
			return ErrInvalidOverride
		}
		if it.MaxNights != nil && *it.MaxNights < 1 {
			return ErrInvalidOverride
		}
		if it.MinNights != nil && it.MaxNights != nil && *it.MinNights > *it.MaxNights {
			return ErrInvalidOverride
		}
		if it.RateOverride != nil {
			if _, err := parseMinor(*it.RateOverride); err != nil {
				return fmt.Errorf("%w: rate_override %q", ErrInvalidOverride, *it.RateOverride)
			}
		}
		if it.RateCurrency != nil {
			cur := strings.ToUpper(strings.TrimSpace(*it.RateCurrency))
			if len(cur) != 3 {
				return fmt.Errorf("%w: rate_currency must be ISO 4217 (3 letters)", ErrInvalidOverride)
			}
			it.RateCurrency = &cur
		}
	}
	return s.repo.UpsertOverrides(ctx, accountID, hotelID, req.Items)
}

// DeleteAvailabilityOverride removes a single (room_type_id, date) override.
func (s *Service) DeleteAvailabilityOverride(ctx context.Context, accountID, hotelID, roomTypeID uuid.UUID, date time.Time) error {
	if date.IsZero() {
		return ErrInvalidDate
	}
	return s.repo.DeleteOverride(ctx, accountID, hotelID, roomTypeID, date)
}

// ----- pricing rules -----

func (s *Service) ListRules(ctx context.Context, accountID, hotelID uuid.UUID) ([]PricingRule, error) {
	return s.repo.ListRules(ctx, accountID, hotelID)
}

func (s *Service) CreateRule(ctx context.Context, accountID, hotelID uuid.UUID, req CreateRuleRequest) (*PricingRule, error) {
	if err := validateRuleInput(req); err != nil {
		return nil, err
	}
	return s.repo.CreateRule(ctx, accountID, hotelID, req)
}

func (s *Service) UpdateRule(ctx context.Context, accountID, hotelID, id uuid.UUID, patch UpdateRuleRequest) (*PricingRule, error) {
	if patch.Name != nil {
		trimmed := strings.TrimSpace(*patch.Name)
		if trimmed == "" || len(trimmed) > nameMaxLen {
			return nil, ErrInvalidRule
		}
		patch.Name = &trimmed
	}
	if patch.ModifierType != nil {
		if !isValidModifierType(*patch.ModifierType) {
			return nil, ErrInvalidRule
		}
	}
	if patch.ModifierValue != nil {
		if _, err := parseMinor(*patch.ModifierValue); err != nil {
			return nil, fmt.Errorf("%w: modifier_value", ErrInvalidRule)
		}
	}
	if patch.MinNights != nil && *patch.MinNights < 1 {
		return nil, ErrInvalidRule
	}
	if patch.MinDaysAhead != nil && *patch.MinDaysAhead < 0 {
		return nil, ErrInvalidRule
	}
	if patch.MaxDaysAhead != nil && *patch.MaxDaysAhead < 0 {
		return nil, ErrInvalidRule
	}
	if patch.DaysOfWeek != nil {
		for _, d := range *patch.DaysOfWeek {
			if d < 1 || d > 7 {
				return nil, ErrInvalidRule
			}
		}
	}
	return s.repo.UpdateRule(ctx, accountID, hotelID, id, patch)
}

func (s *Service) DeleteRule(ctx context.Context, accountID, hotelID, id uuid.UUID) error {
	return s.repo.DeleteRule(ctx, accountID, hotelID, id)
}

// ----- public quote -----

// PublicQuote runs the full engine pipeline for a guest-side preview. The
// hotel must be live; the room_type must belong to that hotel.
//
// Note: no inventory subtraction here. The booking module owns availability
// commitment via SELECT FOR UPDATE — this endpoint is a *price preview*.
func (s *Service) PublicQuote(ctx context.Context, slug string, req QuoteRequest) (*QuoteResponse, error) {
	slug = strings.TrimSpace(strings.ToLower(slug))
	if slug == "" {
		return nil, ErrInvalidRequest
	}
	if req.RoomTypeID == uuid.Nil {
		return nil, ErrInvalidRequest
	}
	if req.Rooms < 1 {
		return nil, ErrInvalidRooms
	}
	if req.Rooms > maxRooms {
		return nil, ErrInvalidRooms
	}
	if err := validateDateRange(req.CheckIn.Time, req.CheckOut.Time, maxRangeDays); err != nil {
		return nil, err
	}

	hotel, err := s.repo.LookupHotelBySlug(ctx, slug)
	if err != nil {
		return nil, err
	}
	rt, err := s.repo.GetRoomTypeForQuote(ctx, hotel.ID, req.RoomTypeID)
	if err != nil {
		return nil, err
	}
	overrides, err := s.repo.listOverridesNoAuth(ctx, hotel.ID, req.CheckIn.Time, req.CheckOut.Time, &rt.ID)
	if err != nil {
		return nil, err
	}
	rules, err := s.repo.listRulesNoAuth(ctx, hotel.ID)
	if err != nil {
		return nil, err
	}

	engineOverrides := map[string]EngineOverride{}
	for _, ov := range overrides {
		if ov.RateOverrideMinor != nil {
			engineOverrides[ov.Date.Format(DateLayout)] = EngineOverride{
				RateOverrideMinor: ov.RateOverrideMinor,
				RateCurrency:      ov.RateCurrency,
			}
		}
	}

	scoped := make([]EngineRule, 0, len(rules))
	for _, pr := range rules {
		if !pr.Enabled {
			continue
		}
		if pr.RoomTypeID != nil && *pr.RoomTypeID != rt.ID {
			continue
		}
		er, err := ToEngineRule(pr)
		if err != nil {
			return nil, err
		}
		scoped = append(scoped, er)
	}

	q, err := CalculateQuote(QuoteInput{
		BaseRateMinor: rt.BaseRateMinor,
		Currency:      rt.BaseCurrency,
		Overrides:     engineOverrides,
		Rules:         scoped,
		CheckIn:       req.CheckIn.Time,
		CheckOut:      req.CheckOut.Time,
		Today:         s.startOfDay(s.now()),
	})
	if err != nil {
		return nil, err
	}

	// Apply rooms multiplier to subtotal and total. Per-night amounts are
	// returned as per-room values (display-friendly).
	subtotal := q.SubtotalMinor * int64(req.Rooms)
	total := q.TotalMinor * int64(req.Rooms)

	return &QuoteResponse{
		HotelID:     hotel.ID,
		RoomTypeID:  rt.ID,
		Rooms:       req.Rooms,
		Nights:      q.Nights,
		Currency:    q.Currency,
		PerNight:    q.PerNight,
		Subtotal:    formatMinor(subtotal),
		Total:       formatMinor(total),
		Adjustments: q.Adjustments,
	}, nil
}

// ----- helpers -----

func (s *Service) startOfDay(t time.Time) time.Time {
	y, m, d := t.UTC().Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func validateDateRange(start, end time.Time, maxDays int) error {
	if start.IsZero() || end.IsZero() {
		return ErrInvalidDate
	}
	if !end.After(start) {
		return ErrInvalidDateRange
	}
	if int(end.Sub(start).Hours()/24) > maxDays {
		return fmt.Errorf("%w: range exceeds %d days", ErrInvalidDateRange, maxDays)
	}
	return nil
}

func validateRuleInput(req CreateRuleRequest) error {
	name := strings.TrimSpace(req.Name)
	if name == "" || len(name) > nameMaxLen {
		return ErrInvalidRule
	}
	if !isValidRuleType(req.RuleType) {
		return ErrInvalidRule
	}
	if !isValidModifierType(req.ModifierType) {
		return ErrInvalidRule
	}
	if _, err := parseMinor(req.ModifierValue); err != nil {
		return fmt.Errorf("%w: modifier_value %q", ErrInvalidRule, req.ModifierValue)
	}
	switch req.RuleType {
	case RuleTypeSeason:
		if req.StartDate == nil || req.EndDate == nil {
			return fmt.Errorf("%w: season requires start_date and end_date", ErrInvalidRule)
		}
		if req.EndDate.Before(req.StartDate.Time) {
			return fmt.Errorf("%w: end_date must be ≥ start_date", ErrInvalidRule)
		}
	case RuleTypeDayOfWeek:
		if len(req.DaysOfWeek) == 0 {
			return fmt.Errorf("%w: day_of_week requires days_of_week", ErrInvalidRule)
		}
		for _, d := range req.DaysOfWeek {
			if d < 1 || d > 7 {
				return fmt.Errorf("%w: day_of_week values must be 1..7", ErrInvalidRule)
			}
		}
	case RuleTypeLengthOfStay:
		if req.MinNights == nil || *req.MinNights < 1 {
			return fmt.Errorf("%w: length_of_stay requires min_nights ≥ 1", ErrInvalidRule)
		}
		if req.MaxNights != nil && *req.MaxNights < *req.MinNights {
			return fmt.Errorf("%w: max_nights must be ≥ min_nights", ErrInvalidRule)
		}
	case RuleTypeAdvancePurchase:
		if req.MinDaysAhead == nil || *req.MinDaysAhead < 0 {
			return fmt.Errorf("%w: advance_purchase requires min_days_ahead ≥ 0", ErrInvalidRule)
		}
		if req.MaxDaysAhead != nil && *req.MaxDaysAhead < *req.MinDaysAhead {
			return fmt.Errorf("%w: max_days_ahead must be ≥ min_days_ahead", ErrInvalidRule)
		}
	}
	return nil
}

func isValidRuleType(s string) bool {
	switch s {
	case RuleTypeSeason, RuleTypeDayOfWeek, RuleTypeLengthOfStay, RuleTypeAdvancePurchase:
		return true
	}
	return false
}

func isValidModifierType(s string) bool {
	switch s {
	case ModifierPercentage, ModifierFixedAmount, ModifierSetValue:
		return true
	}
	return false
}
