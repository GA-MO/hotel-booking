package pricing

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

// helpers ---------------------------------------------------------------------

func intPtr(v int) *int { return &v }

func int64Ptr(v int64) *int64 { return &v }

func timePtr(t time.Time) *time.Time { return &t }

// engineSeed returns a baseline QuoteInput we can mutate per-test.
func engineSeed(checkIn, checkOut string) QuoteInput {
	return QuoteInput{
		BaseRateMinor: 100000, // 1000.00 THB
		Currency:      "THB",
		Overrides:     nil,
		Rules:         nil,
		CheckIn:       mustParse(checkIn),
		CheckOut:      mustParse(checkOut),
		Today:         mustParse("2025-01-01"),
	}
}

func mustParse(s string) time.Time {
	d, err := time.Parse(DateLayout, s)
	if err != nil {
		panic(err)
	}
	return d
}

// ----- formatMinor / parseMinor ----------------------------------------------

func TestFormatMinor(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   int64
		want string
	}{
		{0, "0.00"},
		{1, "0.01"},
		{99, "0.99"},
		{100, "1.00"},
		{150050, "1500.50"},
		{-1, "-0.01"},
		{-1500, "-15.00"},
	}
	for _, c := range cases {
		if got := formatMinor(c.in); got != c.want {
			t.Errorf("formatMinor(%d) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestParseMinor(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in      string
		want    int64
		wantErr bool
	}{
		{"0", 0, false},
		{"0.00", 0, false},
		{"1500", 150000, false},
		{"1500.00", 150000, false},
		{"1500.5", 150050, false},
		{"1500.50", 150050, false},
		{"-10", -1000, false},
		{"-10.5", -1050, false},
		{"+1.25", 125, false},
		{".25", 25, false},
		{"1500.005", 150000, false}, // truncates beyond 2 decimals
		{"", 0, true},
		{"abc", 0, true},
		{"1.2.3", 0, true},
	}
	for _, c := range cases {
		got, err := parseMinor(c.in)
		if c.wantErr {
			if err == nil {
				t.Errorf("parseMinor(%q): want error, got %d", c.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseMinor(%q): unexpected error %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("parseMinor(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}

// ----- daysBetween & isoWeekday ----------------------------------------------

func TestDaysBetween(t *testing.T) {
	t.Parallel()
	cases := []struct {
		a, b string
		want int
	}{
		{"2025-01-01", "2025-01-01", 0},
		{"2025-01-01", "2025-01-02", 1},
		{"2025-01-01", "2025-01-08", 7},
		{"2025-01-31", "2025-02-01", 1},
		{"2025-02-28", "2025-03-01", 1},
		{"2024-02-28", "2024-03-01", 2}, // leap year
		{"2025-01-02", "2025-01-01", -1},
	}
	for _, c := range cases {
		got := daysBetween(mustParse(c.a), mustParse(c.b))
		if got != c.want {
			t.Errorf("daysBetween(%s, %s) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestISOWeekday(t *testing.T) {
	t.Parallel()
	// 2025-01-06 is a Monday → 1; 2025-01-12 Sunday → 7.
	cases := []struct {
		d    string
		want int
	}{
		{"2025-01-06", 1}, // Mon
		{"2025-01-07", 2},
		{"2025-01-08", 3},
		{"2025-01-09", 4},
		{"2025-01-10", 5}, // Fri
		{"2025-01-11", 6}, // Sat
		{"2025-01-12", 7}, // Sun
	}
	for _, c := range cases {
		got := isoWeekday(mustParse(c.d))
		if got != c.want {
			t.Errorf("isoWeekday(%s) = %d, want %d", c.d, got, c.want)
		}
	}
}

// ----- applyModifier ---------------------------------------------------------

func TestApplyModifier(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		amount int64
		rule   EngineRule
		want   int64
	}{
		{"+20% on 1000.00 -> 1200.00", 100000, EngineRule{ModifierType: ModifierPercentage, ModifierMinor: 2000}, 120000},
		{"-10% on 1000.00 -> 900.00", 100000, EngineRule{ModifierType: ModifierPercentage, ModifierMinor: -1000}, 90000},
		{"+50% on 1500.00 -> 2250.00", 150000, EngineRule{ModifierType: ModifierPercentage, ModifierMinor: 5000}, 225000},
		{"fixed +500.00", 100000, EngineRule{ModifierType: ModifierFixedAmount, ModifierMinor: 50000}, 150000},
		{"fixed -200.00", 100000, EngineRule{ModifierType: ModifierFixedAmount, ModifierMinor: -20000}, 80000},
		{"set 2500.00", 100000, EngineRule{ModifierType: ModifierSetValue, ModifierMinor: 250000}, 250000},
		{"unknown modifier is identity", 100000, EngineRule{ModifierType: "bogus", ModifierMinor: 9999}, 100000},
		{"percentage on zero", 0, EngineRule{ModifierType: ModifierPercentage, ModifierMinor: 5000}, 0},
		{"percentage rounding 0.5 satang up", 1, EngineRule{ModifierType: ModifierPercentage, ModifierMinor: 5000}, 2}, // 1 * 1.5 = 1.5 -> rounds to 2
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got := applyModifier(c.amount, c.rule)
			if got != c.want {
				t.Errorf("got %d, want %d", got, c.want)
			}
		})
	}
}

// ----- CalculateQuote: invariants & errors -----------------------------------

func TestCalculateQuote_Errors(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		mutate  func(in *QuoteInput)
		wantErr error
	}{
		{
			"check_out == check_in",
			func(in *QuoteInput) { in.CheckOut = in.CheckIn },
			ErrInvalidDateRange,
		},
		{
			"check_out before check_in",
			func(in *QuoteInput) { in.CheckOut = in.CheckIn.AddDate(0, 0, -1) },
			ErrInvalidDateRange,
		},
		{
			"negative base rate",
			func(in *QuoteInput) { in.BaseRateMinor = -1 },
			ErrInvalidRequest,
		},
		{
			"missing currency",
			func(in *QuoteInput) { in.Currency = "" },
			ErrInvalidRequest,
		},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			in := engineSeed("2025-06-02", "2025-06-05")
			c.mutate(&in)
			_, err := CalculateQuote(in)
			if !errors.Is(err, c.wantErr) {
				t.Fatalf("want %v, got %v", c.wantErr, err)
			}
		})
	}
}

// ----- CalculateQuote: base + scenarios --------------------------------------

func TestCalculateQuote_BaseRateOnly(t *testing.T) {
	t.Parallel()
	// 3-night stay, 1000.00 / night.
	in := engineSeed("2025-06-02", "2025-06-05")
	got, err := CalculateQuote(in)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got.Nights != 3 {
		t.Errorf("nights: want 3, got %d", got.Nights)
	}
	if got.SubtotalMinor != 300000 {
		t.Errorf("subtotal: want 300000, got %d", got.SubtotalMinor)
	}
	if got.TotalMinor != 300000 {
		t.Errorf("total: want 300000, got %d", got.TotalMinor)
	}
	if got.Subtotal != "3000.00" {
		t.Errorf("subtotal string: want 3000.00, got %s", got.Subtotal)
	}
	for _, pn := range got.PerNight {
		if pn.AmountMinor != 100000 {
			t.Errorf("per-night minor: want 100000, got %d (date=%s)", pn.AmountMinor, pn.Date.Format(DateLayout))
		}
		if pn.Currency != "THB" {
			t.Errorf("currency: want THB, got %s", pn.Currency)
		}
	}
}

func TestCalculateQuote_OneNight(t *testing.T) {
	t.Parallel()
	in := engineSeed("2025-06-02", "2025-06-03")
	got, err := CalculateQuote(in)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got.Nights != 1 {
		t.Fatalf("nights: want 1, got %d", got.Nights)
	}
	if got.TotalMinor != 100000 {
		t.Errorf("total: want 100000, got %d", got.TotalMinor)
	}
}

func TestCalculateQuote_WeekendUplift(t *testing.T) {
	t.Parallel()
	// 2025-06-02 is Mon; stay Mon→Sun (5 nights): Mon, Tue, Wed, Thu, Fri.
	// Weekend rule +20% covers Fri, Sat (ISO 5, 6). Stay ends Sun, so check
	// 6/2→6/7 (5 nights). Nights iterated: Mon, Tue, Wed, Thu, Fri.
	in := engineSeed("2025-06-02", "2025-06-07")
	in.Rules = []EngineRule{
		{
			Name:          "Weekend",
			RuleType:      RuleTypeDayOfWeek,
			DaysOfWeek:    []int{5, 6}, // Fri, Sat
			ModifierType:  ModifierPercentage,
			ModifierMinor: 2000,
			Priority:      100,
		},
	}
	got, err := CalculateQuote(in)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	// 4 weekdays at 1000 + 1 weekend at 1200 = 5200.00
	if got.SubtotalMinor != 520000 {
		t.Errorf("subtotal: want 520000, got %d", got.SubtotalMinor)
	}
}

func TestCalculateQuote_WeekendUpliftFullWeek(t *testing.T) {
	t.Parallel()
	// 2025-06-02 (Mon) → 2025-06-09 (next Mon) = 7 nights Mon..Sun.
	// Fri-Sat uplift +20% → 2 nights at 1200, 5 at 1000 = 7400.
	in := engineSeed("2025-06-02", "2025-06-09")
	in.Rules = []EngineRule{
		{
			Name:          "Weekend",
			RuleType:      RuleTypeDayOfWeek,
			DaysOfWeek:    []int{5, 6},
			ModifierType:  ModifierPercentage,
			ModifierMinor: 2000,
		},
	}
	got, _ := CalculateQuote(in)
	if got.SubtotalMinor != 740000 {
		t.Errorf("subtotal: want 740000, got %d", got.SubtotalMinor)
	}
}

func TestCalculateQuote_SeasonPartialOverlap(t *testing.T) {
	t.Parallel()
	// 5 nights: 2025-06-30, 07-01, 07-02, 07-03, 07-04
	// High season 07-01..07-31 covers 4 nights at +50%.
	in := engineSeed("2025-06-30", "2025-07-05")
	in.Rules = []EngineRule{
		{
			Name:          "High Season",
			RuleType:      RuleTypeSeason,
			StartDate:     timePtr(mustParse("2025-07-01")),
			EndDate:       timePtr(mustParse("2025-07-31")),
			ModifierType:  ModifierPercentage,
			ModifierMinor: 5000,
			Priority:      100,
		},
	}
	got, _ := CalculateQuote(in)
	// 1*1000 + 4*1500 = 7000.
	if got.SubtotalMinor != 700000 {
		t.Errorf("subtotal: want 700000, got %d", got.SubtotalMinor)
	}
}

func TestCalculateQuote_SeasonPriorityOrdering(t *testing.T) {
	t.Parallel()
	// Two overlapping seasons:
	//   - Promo (priority 50, -10%) — applies first
	//   - Peak  (priority 100, +50%) — applies second on the discounted base
	// Order: 1000 → 900 (-10%) → 1350 (+50%).
	in := engineSeed("2025-07-15", "2025-07-16")
	in.Rules = []EngineRule{
		{
			Name:          "Peak",
			RuleType:      RuleTypeSeason,
			StartDate:     timePtr(mustParse("2025-07-01")),
			EndDate:       timePtr(mustParse("2025-07-31")),
			ModifierType:  ModifierPercentage,
			ModifierMinor: 5000,
			Priority:      100,
		},
		{
			Name:          "Promo",
			RuleType:      RuleTypeSeason,
			StartDate:     timePtr(mustParse("2025-07-10")),
			EndDate:       timePtr(mustParse("2025-07-20")),
			ModifierType:  ModifierPercentage,
			ModifierMinor: -1000,
			Priority:      50,
		},
	}
	got, _ := CalculateQuote(in)
	if got.TotalMinor != 135000 {
		t.Errorf("total: want 135000, got %d", got.TotalMinor)
	}
}

func TestCalculateQuote_DateOverrideWinsOverRules(t *testing.T) {
	t.Parallel()
	in := engineSeed("2025-07-04", "2025-07-05")
	// Weekend rule would push to 1200, but override fixes the night at 2000.
	in.Rules = []EngineRule{{
		Name:          "Weekend",
		RuleType:      RuleTypeDayOfWeek,
		DaysOfWeek:    []int{5, 6},
		ModifierType:  ModifierPercentage,
		ModifierMinor: 2000,
	}}
	in.Overrides = map[string]EngineOverride{
		"2025-07-04": {RateOverrideMinor: int64Ptr(200000)},
	}
	got, _ := CalculateQuote(in)
	if got.TotalMinor != 200000 {
		t.Errorf("total: want 200000, got %d", got.TotalMinor)
	}
	if got.PerNight[0].AppliedRule[len(got.PerNight[0].AppliedRule)-1] != "date_override" {
		t.Errorf("expected date_override in trace, got %v", got.PerNight[0].AppliedRule)
	}
}

func TestCalculateQuote_LOSDiscount(t *testing.T) {
	t.Parallel()
	// 7-night stay, base 1000/night = 7000; LOS -10% if ≥ 7 → 6300.
	in := engineSeed("2025-09-01", "2025-09-08")
	in.Rules = []EngineRule{{
		Name:          "Weekly stay -10%",
		RuleType:      RuleTypeLengthOfStay,
		MinNights:     intPtr(7),
		ModifierType:  ModifierPercentage,
		ModifierMinor: -1000,
	}}
	got, _ := CalculateQuote(in)
	if got.SubtotalMinor != 700000 {
		t.Errorf("subtotal: want 700000, got %d", got.SubtotalMinor)
	}
	if got.TotalMinor != 630000 {
		t.Errorf("total after LOS: want 630000, got %d", got.TotalMinor)
	}
}

func TestCalculateQuote_LOSDoesNotApplyBelowMin(t *testing.T) {
	t.Parallel()
	in := engineSeed("2025-09-01", "2025-09-04") // 3 nights
	in.Rules = []EngineRule{{
		Name:          "Weekly stay -10%",
		RuleType:      RuleTypeLengthOfStay,
		MinNights:     intPtr(7),
		ModifierType:  ModifierPercentage,
		ModifierMinor: -1000,
	}}
	got, _ := CalculateQuote(in)
	if got.TotalMinor != 300000 {
		t.Errorf("total: want 300000, got %d", got.TotalMinor)
	}
}

func TestCalculateQuote_AdvancePurchase(t *testing.T) {
	t.Parallel()
	// Today = 2025-01-01, check-in 2025-02-15 → 45 days ahead, AP min 30 → applies -10%.
	in := engineSeed("2025-02-15", "2025-02-18")
	in.Today = mustParse("2025-01-01")
	in.Rules = []EngineRule{{
		Name:          "AP-30",
		RuleType:      RuleTypeAdvancePurchase,
		MinDaysAhead:  intPtr(30),
		ModifierType:  ModifierPercentage,
		ModifierMinor: -1000,
	}}
	got, _ := CalculateQuote(in)
	// 3*1000 = 3000 → 2700.
	if got.TotalMinor != 270000 {
		t.Errorf("total: want 270000, got %d", got.TotalMinor)
	}
}

func TestCalculateQuote_AdvancePurchaseNotApplied(t *testing.T) {
	t.Parallel()
	in := engineSeed("2025-01-15", "2025-01-18")
	in.Today = mustParse("2025-01-01") // only 14 days ahead, below min 30.
	in.Rules = []EngineRule{{
		Name:          "AP-30",
		RuleType:      RuleTypeAdvancePurchase,
		MinDaysAhead:  intPtr(30),
		ModifierType:  ModifierPercentage,
		ModifierMinor: -1000,
	}}
	got, _ := CalculateQuote(in)
	if got.TotalMinor != 300000 {
		t.Errorf("total: want 300000, got %d", got.TotalMinor)
	}
}

func TestCalculateQuote_AdvancePurchaseMaxDaysCutoff(t *testing.T) {
	t.Parallel()
	// 100 days ahead — above max 60, so rule does NOT apply.
	in := engineSeed("2025-04-10", "2025-04-13")
	in.Today = mustParse("2025-01-01")
	in.Rules = []EngineRule{{
		Name:          "AP narrow",
		RuleType:      RuleTypeAdvancePurchase,
		MinDaysAhead:  intPtr(30),
		MaxDaysAhead:  intPtr(60),
		ModifierType:  ModifierPercentage,
		ModifierMinor: -1500,
	}}
	got, _ := CalculateQuote(in)
	if got.TotalMinor != 300000 {
		t.Errorf("total: want 300000, got %d", got.TotalMinor)
	}
}

func TestCalculateQuote_FixedAmountModifier(t *testing.T) {
	t.Parallel()
	// 2 nights, base 1000. Add +500 per night via day_of_week rule covering both nights.
	// 2025-06-02 Mon, 2025-06-03 Tue.
	in := engineSeed("2025-06-02", "2025-06-04")
	in.Rules = []EngineRule{{
		Name:          "MonTueExtra",
		RuleType:      RuleTypeDayOfWeek,
		DaysOfWeek:    []int{1, 2},
		ModifierType:  ModifierFixedAmount,
		ModifierMinor: 50000,
	}}
	got, _ := CalculateQuote(in)
	// 2 * (1000+500) = 3000.
	if got.SubtotalMinor != 300000 {
		t.Errorf("subtotal: want 300000, got %d", got.SubtotalMinor)
	}
}

func TestCalculateQuote_SetValueModifier(t *testing.T) {
	t.Parallel()
	in := engineSeed("2025-06-02", "2025-06-03")
	in.Rules = []EngineRule{{
		Name:          "Promo flat",
		RuleType:      RuleTypeSeason,
		StartDate:     timePtr(mustParse("2025-06-01")),
		EndDate:       timePtr(mustParse("2025-06-30")),
		ModifierType:  ModifierSetValue,
		ModifierMinor: 80000,
	}}
	got, _ := CalculateQuote(in)
	if got.TotalMinor != 80000 {
		t.Errorf("total: want 80000, got %d", got.TotalMinor)
	}
}

func TestCalculateQuote_CombinedAllFour(t *testing.T) {
	t.Parallel()
	// 7-night stay starting Sat 2025-07-05 → 2025-07-12 (Sat).
	// Nights iterated: 07-05 Sat, 07-06 Sun, 07-07 Mon, ..., 07-11 Fri.
	// Weekend +20% on Fri/Sat → 07-05 Sat, 07-11 Fri → 2 nights.
	// Season +50% covers all of July → applies to every night.
	// Order on each night: base → weekend → season.
	//   Weekend night: 1000 → 1200 → 1800.
	//   Non-weekend:   1000 → 1000 → 1500.
	// Subtotal: 2*1800 + 5*1500 = 3600 + 7500 = 11100.
	// LOS -10% on ≥7 → 11100 * 0.9 = 9990.
	// AP -5% on ≥30 days ahead (today 2025-01-01 → 185 days) → 9990 * 0.95 = 9490.50.
	in := engineSeed("2025-07-05", "2025-07-12")
	in.Today = mustParse("2025-01-01")
	in.Rules = []EngineRule{
		{
			Name:          "Weekend",
			RuleType:      RuleTypeDayOfWeek,
			DaysOfWeek:    []int{5, 6},
			ModifierType:  ModifierPercentage,
			ModifierMinor: 2000,
		},
		{
			Name:          "Peak",
			RuleType:      RuleTypeSeason,
			StartDate:     timePtr(mustParse("2025-07-01")),
			EndDate:       timePtr(mustParse("2025-07-31")),
			ModifierType:  ModifierPercentage,
			ModifierMinor: 5000,
		},
		{
			Name:          "Weekly -10%",
			RuleType:      RuleTypeLengthOfStay,
			MinNights:     intPtr(7),
			ModifierType:  ModifierPercentage,
			ModifierMinor: -1000,
		},
		{
			Name:          "Early-bird -5%",
			RuleType:      RuleTypeAdvancePurchase,
			MinDaysAhead:  intPtr(30),
			ModifierType:  ModifierPercentage,
			ModifierMinor: -500,
		},
	}
	got, err := CalculateQuote(in)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got.SubtotalMinor != 1110000 {
		t.Errorf("subtotal: want 1110000, got %d", got.SubtotalMinor)
	}
	// 1110000 * 0.9 = 999000; *0.95 = 949050.
	if got.TotalMinor != 949050 {
		t.Errorf("total: want 949050, got %d", got.TotalMinor)
	}
	if len(got.Adjustments) != 2 {
		t.Errorf("expected 2 adjustments (LOS + AP), got %d: %v", len(got.Adjustments), got.Adjustments)
	}
}

func TestCalculateQuote_DisabledRulesIgnored_HandledByCaller(t *testing.T) {
	t.Parallel()
	// The engine itself does not consult `Enabled` — the caller filters. Confirm
	// that when the caller passes no rules, base wins.
	in := engineSeed("2025-06-02", "2025-06-03")
	got, _ := CalculateQuote(in)
	if got.TotalMinor != 100000 {
		t.Errorf("total: want 100000, got %d", got.TotalMinor)
	}
}

func TestCalculateQuote_NegativeRateClampedToZero(t *testing.T) {
	t.Parallel()
	// Fixed -1500 on a 1000 base would go negative; engine clamps to 0.
	in := engineSeed("2025-06-02", "2025-06-03")
	in.Rules = []EngineRule{{
		Name:          "Hostile",
		RuleType:      RuleTypeDayOfWeek,
		DaysOfWeek:    []int{1},
		ModifierType:  ModifierFixedAmount,
		ModifierMinor: -150000,
	}}
	got, _ := CalculateQuote(in)
	if got.TotalMinor != 0 {
		t.Errorf("total: want 0 (clamped), got %d", got.TotalMinor)
	}
}

func TestCalculateQuote_GlobalAndScopedRulesCoexist(t *testing.T) {
	t.Parallel()
	// Engine doesn't filter by room_type — caller does. Confirm two rules
	// applied independently are additive (multiplicative in compound).
	in := engineSeed("2025-06-02", "2025-06-03") // Mon, 1 night
	in.Rules = []EngineRule{
		{
			Name:          "A",
			RuleType:      RuleTypeDayOfWeek,
			DaysOfWeek:    []int{1},
			ModifierType:  ModifierPercentage,
			ModifierMinor: 1000, // +10%
		},
		{
			Name:          "B",
			RuleType:      RuleTypeDayOfWeek,
			DaysOfWeek:    []int{1},
			ModifierType:  ModifierPercentage,
			ModifierMinor: 1000, // +10%
			Priority:      200,
		},
	}
	got, _ := CalculateQuote(in)
	// 1000 → 1100 → 1210.
	if got.TotalMinor != 121000 {
		t.Errorf("total: want 121000, got %d", got.TotalMinor)
	}
}

func TestCalculateQuote_RuleTraceCapturesAppliedNames(t *testing.T) {
	t.Parallel()
	in := engineSeed("2025-07-05", "2025-07-06") // Sat, July
	in.Rules = []EngineRule{
		{
			Name:          "Weekend",
			RuleType:      RuleTypeDayOfWeek,
			DaysOfWeek:    []int{5, 6},
			ModifierType:  ModifierPercentage,
			ModifierMinor: 2000,
		},
		{
			Name:          "Peak",
			RuleType:      RuleTypeSeason,
			StartDate:     timePtr(mustParse("2025-07-01")),
			EndDate:       timePtr(mustParse("2025-07-31")),
			ModifierType:  ModifierPercentage,
			ModifierMinor: 5000,
		},
	}
	got, _ := CalculateQuote(in)
	if len(got.PerNight) != 1 {
		t.Fatalf("expected 1 night, got %d", len(got.PerNight))
	}
	applied := got.PerNight[0].AppliedRule
	if len(applied) != 2 || applied[0] != "Weekend" || applied[1] != "Peak" {
		t.Errorf("expected trace [Weekend, Peak], got %v", applied)
	}
}

func TestCalculateQuote_OverrideZeroRate(t *testing.T) {
	t.Parallel()
	in := engineSeed("2025-06-02", "2025-06-03")
	in.Overrides = map[string]EngineOverride{
		"2025-06-02": {RateOverrideMinor: int64Ptr(0)},
	}
	got, _ := CalculateQuote(in)
	if got.TotalMinor != 0 {
		t.Errorf("total: want 0, got %d", got.TotalMinor)
	}
}

func TestCalculateQuote_BackToBackAPAndLOSStack(t *testing.T) {
	t.Parallel()
	// 10 nights, 1000/night = 10000.
	// LOS -10% if ≥7 → 9000.
	// AP -10% if ≥30 → 8100.
	in := engineSeed("2025-03-01", "2025-03-11")
	in.Today = mustParse("2025-01-01")
	in.Rules = []EngineRule{
		{
			Name:          "Weekly",
			RuleType:      RuleTypeLengthOfStay,
			MinNights:     intPtr(7),
			ModifierType:  ModifierPercentage,
			ModifierMinor: -1000,
		},
		{
			Name:          "AP",
			RuleType:      RuleTypeAdvancePurchase,
			MinDaysAhead:  intPtr(30),
			ModifierType:  ModifierPercentage,
			ModifierMinor: -1000,
		},
	}
	got, _ := CalculateQuote(in)
	if got.TotalMinor != 810000 {
		t.Errorf("total: want 810000, got %d", got.TotalMinor)
	}
}

// ----- ToEngineRule -----------------------------------------------------------

func TestToEngineRule_PercentageOK(t *testing.T) {
	t.Parallel()
	r := PricingRule{
		ID:            uuidNew(t),
		Name:          "W",
		RuleType:      RuleTypeDayOfWeek,
		DaysOfWeek:    []int{5, 6},
		ModifierType:  ModifierPercentage,
		ModifierValue: "20",
		Priority:      100,
	}
	er, err := ToEngineRule(r)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if er.ModifierMinor != 2000 {
		t.Errorf("want 2000, got %d", er.ModifierMinor)
	}
}

func TestToEngineRule_InvalidModifierType(t *testing.T) {
	t.Parallel()
	r := PricingRule{
		ID:            uuidNew(t),
		Name:          "Bad",
		RuleType:      RuleTypeDayOfWeek,
		DaysOfWeek:    []int{1},
		ModifierType:  "bogus",
		ModifierValue: "10",
	}
	if _, err := ToEngineRule(r); !errors.Is(err, ErrInvalidRule) {
		t.Fatalf("want ErrInvalidRule, got %v", err)
	}
}

func TestToEngineRule_InvalidModifierValue(t *testing.T) {
	t.Parallel()
	r := PricingRule{
		ID:            uuidNew(t),
		Name:          "Bad",
		RuleType:      RuleTypeSeason,
		StartDate:     &Date{mustParse("2025-01-01")},
		EndDate:       &Date{mustParse("2025-12-31")},
		ModifierType:  ModifierPercentage,
		ModifierValue: "not-a-number",
	}
	if _, err := ToEngineRule(r); err == nil {
		t.Fatalf("expected error")
	}
}

func uuidNew(t *testing.T) uuid.UUID {
	t.Helper()
	return uuid.New()
}
