package pricing

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestValidateDateRange(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		start   string
		end     string
		max     int
		wantErr error
	}{
		{"valid 3 nights", "2025-06-01", "2025-06-04", 365, nil},
		{"same day", "2025-06-01", "2025-06-01", 365, ErrInvalidDateRange},
		{"reversed", "2025-06-04", "2025-06-01", 365, ErrInvalidDateRange},
		{"exceeds max", "2025-01-01", "2026-01-02", 365, ErrInvalidDateRange},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			err := validateDateRange(mustParse(c.start), mustParse(c.end), c.max)
			if c.wantErr == nil && err != nil {
				t.Fatalf("want nil, got %v", err)
			}
			if c.wantErr != nil && !errors.Is(err, c.wantErr) {
				t.Fatalf("want %v, got %v", c.wantErr, err)
			}
		})
	}
}

func TestValidateDateRange_ZeroDates(t *testing.T) {
	t.Parallel()
	if err := validateDateRange(time.Time{}, mustParse("2025-01-02"), 365); !errors.Is(err, ErrInvalidDate) {
		t.Fatalf("want ErrInvalidDate, got %v", err)
	}
	if err := validateDateRange(mustParse("2025-01-01"), time.Time{}, 365); !errors.Is(err, ErrInvalidDate) {
		t.Fatalf("want ErrInvalidDate, got %v", err)
	}
}

func TestIsValidRuleType(t *testing.T) {
	t.Parallel()
	cases := map[string]bool{
		RuleTypeSeason:          true,
		RuleTypeDayOfWeek:       true,
		RuleTypeLengthOfStay:    true,
		RuleTypeAdvancePurchase: true,
		"":                      false,
		"bogus":                 false,
	}
	for in, want := range cases {
		if got := isValidRuleType(in); got != want {
			t.Errorf("isValidRuleType(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestIsValidModifierType(t *testing.T) {
	t.Parallel()
	cases := map[string]bool{
		ModifierPercentage:  true,
		ModifierFixedAmount: true,
		ModifierSetValue:    true,
		"":                  false,
		"bogus":             false,
	}
	for in, want := range cases {
		if got := isValidModifierType(in); got != want {
			t.Errorf("isValidModifierType(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestValidateRuleInput(t *testing.T) {
	t.Parallel()
	good := CreateRuleRequest{
		Name:          "Weekend",
		RuleType:      RuleTypeDayOfWeek,
		DaysOfWeek:    []int{5, 6},
		ModifierType:  ModifierPercentage,
		ModifierValue: "20",
	}

	cases := []struct {
		name    string
		mutate  func(r *CreateRuleRequest)
		wantErr error
	}{
		{"valid weekend", func(r *CreateRuleRequest) {}, nil},
		{"empty name", func(r *CreateRuleRequest) { r.Name = "" }, ErrInvalidRule},
		{"name too long", func(r *CreateRuleRequest) { r.Name = strings.Repeat("a", 121) }, ErrInvalidRule},
		{"unknown rule_type", func(r *CreateRuleRequest) { r.RuleType = "x" }, ErrInvalidRule},
		{"unknown modifier_type", func(r *CreateRuleRequest) { r.ModifierType = "x" }, ErrInvalidRule},
		{"non-numeric modifier_value", func(r *CreateRuleRequest) { r.ModifierValue = "abc" }, ErrInvalidRule},
		{"day_of_week without days", func(r *CreateRuleRequest) { r.DaysOfWeek = nil }, ErrInvalidRule},
		{"day_of_week with bad day", func(r *CreateRuleRequest) { r.DaysOfWeek = []int{0} }, ErrInvalidRule},
		{"day_of_week with bad day 8", func(r *CreateRuleRequest) { r.DaysOfWeek = []int{8} }, ErrInvalidRule},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			r := good
			r.DaysOfWeek = append([]int(nil), good.DaysOfWeek...)
			c.mutate(&r)
			err := validateRuleInput(r)
			if c.wantErr == nil && err != nil {
				t.Fatalf("want nil, got %v", err)
			}
			if c.wantErr != nil && !errors.Is(err, c.wantErr) {
				t.Fatalf("want %v, got %v", c.wantErr, err)
			}
		})
	}
}

func TestValidateRuleInput_Season(t *testing.T) {
	t.Parallel()
	base := CreateRuleRequest{
		Name:          "High",
		RuleType:      RuleTypeSeason,
		ModifierType:  ModifierPercentage,
		ModifierValue: "50",
	}
	t.Run("missing dates", func(t *testing.T) {
		t.Parallel()
		if err := validateRuleInput(base); !errors.Is(err, ErrInvalidRule) {
			t.Fatalf("want ErrInvalidRule, got %v", err)
		}
	})
	t.Run("end before start", func(t *testing.T) {
		t.Parallel()
		r := base
		s := Date{mustParse("2025-08-01")}
		e := Date{mustParse("2025-07-01")}
		r.StartDate = &s
		r.EndDate = &e
		if err := validateRuleInput(r); !errors.Is(err, ErrInvalidRule) {
			t.Fatalf("want ErrInvalidRule, got %v", err)
		}
	})
	t.Run("valid", func(t *testing.T) {
		t.Parallel()
		r := base
		s := Date{mustParse("2025-07-01")}
		e := Date{mustParse("2025-08-31")}
		r.StartDate = &s
		r.EndDate = &e
		if err := validateRuleInput(r); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestValidateRuleInput_LOS(t *testing.T) {
	t.Parallel()
	base := CreateRuleRequest{
		Name:          "Weekly",
		RuleType:      RuleTypeLengthOfStay,
		ModifierType:  ModifierPercentage,
		ModifierValue: "-10",
	}
	t.Run("missing min_nights", func(t *testing.T) {
		t.Parallel()
		if err := validateRuleInput(base); !errors.Is(err, ErrInvalidRule) {
			t.Fatalf("want ErrInvalidRule, got %v", err)
		}
	})
	t.Run("min_nights < 1", func(t *testing.T) {
		t.Parallel()
		r := base
		r.MinNights = intPtr(0)
		if err := validateRuleInput(r); !errors.Is(err, ErrInvalidRule) {
			t.Fatalf("want ErrInvalidRule, got %v", err)
		}
	})
	t.Run("max < min", func(t *testing.T) {
		t.Parallel()
		r := base
		r.MinNights = intPtr(7)
		r.MaxNights = intPtr(3)
		if err := validateRuleInput(r); !errors.Is(err, ErrInvalidRule) {
			t.Fatalf("want ErrInvalidRule, got %v", err)
		}
	})
	t.Run("valid", func(t *testing.T) {
		t.Parallel()
		r := base
		r.MinNights = intPtr(7)
		if err := validateRuleInput(r); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestValidateRuleInput_AdvancePurchase(t *testing.T) {
	t.Parallel()
	base := CreateRuleRequest{
		Name:          "AP",
		RuleType:      RuleTypeAdvancePurchase,
		ModifierType:  ModifierPercentage,
		ModifierValue: "-10",
	}
	t.Run("missing min_days_ahead", func(t *testing.T) {
		t.Parallel()
		if err := validateRuleInput(base); !errors.Is(err, ErrInvalidRule) {
			t.Fatalf("want ErrInvalidRule, got %v", err)
		}
	})
	t.Run("min_days_ahead negative", func(t *testing.T) {
		t.Parallel()
		r := base
		r.MinDaysAhead = intPtr(-1)
		if err := validateRuleInput(r); !errors.Is(err, ErrInvalidRule) {
			t.Fatalf("want ErrInvalidRule, got %v", err)
		}
	})
	t.Run("max < min", func(t *testing.T) {
		t.Parallel()
		r := base
		r.MinDaysAhead = intPtr(30)
		r.MaxDaysAhead = intPtr(7)
		if err := validateRuleInput(r); !errors.Is(err, ErrInvalidRule) {
			t.Fatalf("want ErrInvalidRule, got %v", err)
		}
	})
	t.Run("valid", func(t *testing.T) {
		t.Parallel()
		r := base
		r.MinDaysAhead = intPtr(30)
		if err := validateRuleInput(r); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestDateUnmarshalJSON(t *testing.T) {
	t.Parallel()
	var d Date
	if err := d.UnmarshalJSON([]byte(`"2025-06-15"`)); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if d.Format(DateLayout) != "2025-06-15" {
		t.Errorf("got %s, want 2025-06-15", d.Format(DateLayout))
	}
	var bad Date
	if err := bad.UnmarshalJSON([]byte(`"not-a-date"`)); !errors.Is(err, ErrInvalidDate) {
		t.Errorf("want ErrInvalidDate, got %v", err)
	}
	var nullD Date
	if err := nullD.UnmarshalJSON([]byte(`null`)); err != nil {
		t.Errorf("null should be no-op, got %v", err)
	}
}

func TestDateMarshalJSON(t *testing.T) {
	t.Parallel()
	d := Date{mustParse("2025-06-15")}
	b, err := d.MarshalJSON()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(b) != `"2025-06-15"` {
		t.Errorf("got %s", string(b))
	}
	// Zero date renders as null.
	zb, _ := Date{}.MarshalJSON()
	if string(zb) != "null" {
		t.Errorf("zero date: got %s, want null", string(zb))
	}
}

func TestParseDate(t *testing.T) {
	t.Parallel()
	d, err := ParseDate("2025-06-15")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if d.Format(DateLayout) != "2025-06-15" {
		t.Errorf("got %s", d.Format(DateLayout))
	}
	if _, err := ParseDate("xxx"); !errors.Is(err, ErrInvalidDate) {
		t.Errorf("want ErrInvalidDate, got %v", err)
	}
}
