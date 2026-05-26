package pricing

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Engine inputs ---------------------------------------------------------------

// EngineOverride is the lean override shape the engine consumes (no DB types).
type EngineOverride struct {
	RateOverrideMinor *int64
	RateCurrency      *string
}

// EngineRule is the lean rule shape the engine consumes.
type EngineRule struct {
	ID            string
	RuleType      string
	StartDate     *time.Time
	EndDate       *time.Time
	DaysOfWeek    []int // ISO 1..7 Mon..Sun
	ModifierType  string
	ModifierMinor int64 // for fixed_amount/set_value: minor units; for percentage: scaled by 100 (so 20% -> 2000)
	IsPercentage  bool  // true => ModifierMinor is hundredths-of-percent
	MinNights     *int
	MaxNights     *int
	MinDaysAhead  *int
	MaxDaysAhead  *int
	Priority      int
	Name          string
}

// QuoteInput collects every parameter the engine needs.
type QuoteInput struct {
	BaseRateMinor int64
	Currency      string
	Overrides     map[string]EngineOverride // key: date.Format(DateLayout)
	Rules         []EngineRule
	CheckIn       time.Time
	CheckOut      time.Time
	Today         time.Time // for advance-purchase calculations
}

// CalculateQuote applies the calculation order from plan.md §8.1:
//
//  1. base_rate (from room_type)
//  2. day_of_week modifier
//  3. season modifier (multiple seasons resolved in priority ascending order)
//  4. date_override rate_override REPLACES the computed rate entirely
//  5. sum per-night rates → subtotal
//  6. length_of_stay modifier (applied to subtotal once)
//  7. advance_purchase modifier (applied to subtotal once)
//
// Money is in minor units (THB satang). Caller is responsible for snapshotting
// the currency on booking creation.
func CalculateQuote(in QuoteInput) (Quote, error) {
	if !in.CheckOut.After(in.CheckIn) {
		return Quote{}, ErrInvalidDateRange
	}
	if in.BaseRateMinor < 0 {
		return Quote{}, ErrInvalidRequest
	}
	if strings.TrimSpace(in.Currency) == "" {
		return Quote{}, ErrInvalidRequest
	}

	// Stable, deterministic per-night iteration.
	nights := daysBetween(in.CheckIn, in.CheckOut)
	if nights == 0 {
		return Quote{}, ErrInvalidDateRange
	}

	// Pre-classify rules so the per-night loop stays cheap.
	var (
		dowRules    []EngineRule
		seasonRules []EngineRule
		losRules    []EngineRule
		apRules     []EngineRule
	)
	for _, r := range in.Rules {
		switch r.RuleType {
		case RuleTypeDayOfWeek:
			dowRules = append(dowRules, r)
		case RuleTypeSeason:
			seasonRules = append(seasonRules, r)
		case RuleTypeLengthOfStay:
			losRules = append(losRules, r)
		case RuleTypeAdvancePurchase:
			apRules = append(apRules, r)
		}
	}
	// Seasons applied in ascending priority — lower priority = applied first.
	sort.SliceStable(seasonRules, func(i, j int) bool {
		return seasonRules[i].Priority < seasonRules[j].Priority
	})
	sort.SliceStable(dowRules, func(i, j int) bool {
		return dowRules[i].Priority < dowRules[j].Priority
	})

	perNight := make([]DailyRate, 0, nights)
	var subtotal int64

	for n := 0; n < nights; n++ {
		night := in.CheckIn.AddDate(0, 0, n)
		nightKey := night.Format(DateLayout)
		rate := in.BaseRateMinor
		applied := make([]string, 0, 4)

		// 2. day_of_week (ISO weekday)
		iso := isoWeekday(night)
		for _, r := range dowRules {
			if containsInt(r.DaysOfWeek, iso) {
				rate = applyModifier(rate, r)
				applied = append(applied, r.Name)
			}
		}

		// 3. season — date in [start, end] inclusive
		for _, r := range seasonRules {
			if r.StartDate == nil || r.EndDate == nil {
				continue
			}
			if !night.Before(*r.StartDate) && !night.After(*r.EndDate) {
				rate = applyModifier(rate, r)
				applied = append(applied, r.Name)
			}
		}

		// 4. date override: rate_override REPLACES whatever rules computed.
		if ov, ok := in.Overrides[nightKey]; ok {
			if ov.RateOverrideMinor != nil {
				rate = *ov.RateOverrideMinor
				applied = append(applied, "date_override")
			}
		}

		if rate < 0 {
			rate = 0
		}
		perNight = append(perNight, DailyRate{
			Date:        Date{night},
			AmountMinor: rate,
			Amount:      formatMinor(rate),
			Currency:    in.Currency,
			AppliedRule: applied,
		})
		subtotal += rate
	}

	total := subtotal
	adjustments := []string{}

	// 5. length_of_stay
	for _, r := range losRules {
		if r.MinNights == nil {
			continue
		}
		if nights < *r.MinNights {
			continue
		}
		if r.MaxNights != nil && nights > *r.MaxNights {
			continue
		}
		total = applyModifier(total, r)
		adjustments = append(adjustments, fmt.Sprintf("%s (LOS %d nights)", r.Name, nights))
	}

	// 6. advance_purchase
	daysAhead := daysBetween(in.Today, in.CheckIn)
	if in.CheckIn.Before(in.Today) {
		daysAhead = 0
	}
	for _, r := range apRules {
		if r.MinDaysAhead == nil {
			continue
		}
		if daysAhead < *r.MinDaysAhead {
			continue
		}
		if r.MaxDaysAhead != nil && daysAhead > *r.MaxDaysAhead {
			continue
		}
		total = applyModifier(total, r)
		adjustments = append(adjustments, fmt.Sprintf("%s (AP %d days)", r.Name, daysAhead))
	}

	if total < 0 {
		total = 0
	}

	q := Quote{
		Nights:        nights,
		Currency:      in.Currency,
		PerNight:      perNight,
		SubtotalMinor: subtotal,
		Subtotal:      formatMinor(subtotal),
		TotalMinor:    total,
		Total:         formatMinor(total),
	}
	if len(adjustments) > 0 {
		q.Adjustments = adjustments
	}
	return q, nil
}

// applyModifier mutates a money amount per the rule's modifier type.
//
//   - percentage: new = old * (1 + value/100). value is in *percent*, stored as
//     hundredths-of-percent in ModifierMinor so 20% == 2000.
//     We compute: new = old * (10000 + ModifierMinor) / 10000, rounded half-up.
//   - fixed_amount: new = old + ModifierMinor (minor units)
//   - set_value:    new = ModifierMinor
func applyModifier(amount int64, r EngineRule) int64 {
	switch r.ModifierType {
	case ModifierPercentage:
		// new = round_half_up(amount * (10000 + delta) / 10000).
		// ModifierMinor is hundredths-of-percent: 20% -> 2000, -10.5% -> -1050.
		num := amount * (10000 + r.ModifierMinor)
		if num >= 0 {
			return (num + 5000) / 10000
		}
		return -((-num + 5000) / 10000)
	case ModifierFixedAmount:
		return amount + r.ModifierMinor
	case ModifierSetValue:
		return r.ModifierMinor
	default:
		return amount
	}
}

func daysBetween(a, b time.Time) int {
	// Strip time-of-day so partial-day diffs don't bias the count.
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	a0 := time.Date(ay, am, ad, 0, 0, 0, 0, time.UTC)
	b0 := time.Date(by, bm, bd, 0, 0, 0, 0, time.UTC)
	return int(b0.Sub(a0).Hours() / 24)
}

// isoWeekday returns 1..7 for Mon..Sun, mirroring Postgres EXTRACT(ISODOW).
func isoWeekday(t time.Time) int {
	w := int(t.Weekday()) // Sunday=0..Saturday=6
	if w == 0 {
		return 7
	}
	return w
}

func containsInt(xs []int, x int) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

// formatMinor converts minor units to a "1500.00"-style decimal string. We
// avoid floats entirely to stay byte-identical to NUMERIC(10,2) round-trips.
func formatMinor(n int64) string {
	neg := n < 0
	if neg {
		n = -n
	}
	s := fmt.Sprintf("%d.%02d", n/100, n%100)
	if neg {
		return "-" + s
	}
	return s
}

// parseMinor converts a "1500.00" decimal string into int64 minor units.
// Accepts an optional sign and up to two fractional digits. Anything else is an
// error.
func parseMinor(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, ErrInvalidRequest
	}
	neg := false
	if s[0] == '+' || s[0] == '-' {
		neg = s[0] == '-'
		s = s[1:]
	}
	intPart, fracPart, hasFrac := strings.Cut(s, ".")
	if intPart == "" && !hasFrac {
		return 0, ErrInvalidRequest
	}
	if intPart == "" {
		intPart = "0"
	}
	iv, err := strconv.ParseInt(intPart, 10, 64)
	if err != nil {
		return 0, ErrInvalidRequest
	}
	var fv int64
	if hasFrac {
		switch len(fracPart) {
		case 0:
			fv = 0
		case 1:
			n, err := strconv.ParseInt(fracPart, 10, 64)
			if err != nil {
				return 0, ErrInvalidRequest
			}
			fv = n * 10
		case 2:
			n, err := strconv.ParseInt(fracPart, 10, 64)
			if err != nil {
				return 0, ErrInvalidRequest
			}
			fv = n
		default:
			// Truncate beyond 2 decimals — we already enforce NUMERIC(10,2) at DB.
			n, err := strconv.ParseInt(fracPart[:2], 10, 64)
			if err != nil {
				return 0, ErrInvalidRequest
			}
			fv = n
		}
	}
	result := iv*100 + fv
	if neg {
		result = -result
	}
	return result, nil
}

// parsePercentageMinor converts a "20" or "-10.5" percent string into the
// hundredths-of-percent integer the engine uses (20 -> 2000, -10.5 -> -1050).
func parsePercentageMinor(s string) (int64, error) {
	// percentage value uses 2 decimal places in NUMERIC(10,2) on the DB —
	// percent of percent semantics. We store percent value in hundredths so
	// applyModifier can multiply integers safely.
	return parseMinor(s)
}

// ToEngineRule materialises a DB-side PricingRule into the engine's input shape.
func ToEngineRule(r PricingRule) (EngineRule, error) {
	er := EngineRule{
		ID:           r.ID.String(),
		Name:         r.Name,
		RuleType:     r.RuleType,
		DaysOfWeek:   append([]int(nil), r.DaysOfWeek...),
		ModifierType: r.ModifierType,
		MinNights:    r.MinNights,
		MaxNights:    r.MaxNights,
		MinDaysAhead: r.MinDaysAhead,
		MaxDaysAhead: r.MaxDaysAhead,
		Priority:     r.Priority,
	}
	if r.StartDate != nil {
		t := r.StartDate.Time
		er.StartDate = &t
	}
	if r.EndDate != nil {
		t := r.EndDate.Time
		er.EndDate = &t
	}
	switch r.ModifierType {
	case ModifierPercentage:
		v, err := parsePercentageMinor(r.ModifierValue)
		if err != nil {
			return EngineRule{}, err
		}
		er.ModifierMinor = v
		er.IsPercentage = true
	case ModifierFixedAmount, ModifierSetValue:
		v, err := parseMinor(r.ModifierValue)
		if err != nil {
			return EngineRule{}, err
		}
		er.ModifierMinor = v
	default:
		return EngineRule{}, ErrInvalidRule
	}
	return er, nil
}
