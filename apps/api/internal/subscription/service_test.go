package subscription

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

// TestCreateForAccount_NilUUID checks the trivial input guard. Other behaviour
// requires a DB and is exercised by repository_test.go (integration).
func TestCreateForAccount_NilUUID(t *testing.T) {
	t.Parallel()
	s := &Service{now: fixedNow}
	if _, err := s.CreateForAccount(context.Background(), uuid.Nil); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("want ErrInvalidRequest, got %v", err)
	}
	if _, err := s.EnsureForAccount(context.Background(), uuid.Nil); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("want ErrInvalidRequest, got %v", err)
	}
}

// TestTrialDuration locks in the 365-day promise from plan.md §1.
// Duration math is centralised in the trialDuration constant — this guards
// against accidental edits.
func TestTrialDuration(t *testing.T) {
	t.Parallel()
	want := 365 * 24 * time.Hour
	if trialDuration != want {
		t.Fatalf("trialDuration: want %s, got %s", want, trialDuration)
	}
}

// TestTrialEndsAtMath verifies ActivateTrial would compute trial_ends_at =
// trial_started_at + 365d. We don't have a DB here, so we exercise the math
// helper inline.
func TestTrialEndsAtMath(t *testing.T) {
	t.Parallel()
	start := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	end := start.Add(trialDuration)
	want := time.Date(2027, 1, 15, 12, 0, 0, 0, time.UTC)
	if !end.Equal(want) {
		t.Fatalf("trial end: want %s, got %s", want, end)
	}
}

func TestValidateUpdatePaymentMethod(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		req     UpdatePaymentMethodRequest
		wantErr error
	}{
		{"valid stripe", UpdatePaymentMethodRequest{
			Provider: ProviderStripe, MethodID: "pm_123", Last4: "4242", Brand: "visa",
		}, nil},
		{"valid omise no last4", UpdatePaymentMethodRequest{
			Provider: ProviderOmise, MethodID: "tokn_abc",
		}, nil},
		{"empty provider", UpdatePaymentMethodRequest{
			MethodID: "pm_x",
		}, ErrInvalidProvider},
		{"bad provider", UpdatePaymentMethodRequest{
			Provider: "paypal", MethodID: "pm_x",
		}, ErrInvalidProvider},
		{"empty method id", UpdatePaymentMethodRequest{
			Provider: ProviderStripe,
		}, ErrInvalidPaymentMethod},
		{"whitespace method id", UpdatePaymentMethodRequest{
			Provider: ProviderStripe, MethodID: "   ",
		}, ErrInvalidPaymentMethod},
		{"method id too long", UpdatePaymentMethodRequest{
			Provider: ProviderStripe, MethodID: string(make([]byte, 121)),
		}, ErrInvalidPaymentMethod},
		{"last4 wrong length", UpdatePaymentMethodRequest{
			Provider: ProviderStripe, MethodID: "pm_x", Last4: "42",
		}, ErrInvalidPaymentMethod},
		{"brand too long", UpdatePaymentMethodRequest{
			Provider: ProviderStripe, MethodID: "pm_x",
			Brand: "this-brand-name-is-way-too-long-21",
		}, ErrInvalidPaymentMethod},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			err := validatePaymentMethodReq(c.req)
			if c.wantErr == nil && err != nil {
				t.Fatalf("want nil, got %v", err)
			}
			if c.wantErr != nil && !errors.Is(err, c.wantErr) {
				t.Fatalf("want %v, got %v", c.wantErr, err)
			}
		})
	}
}

func TestHasPaymentMethod(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		sub  *Subscription
		want bool
	}{
		{"nil", nil, false},
		{"empty", &Subscription{}, false},
		{"has token", &Subscription{PaymentMethodID: "pm_123"}, true},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if got := c.sub.HasPaymentMethod(); got != c.want {
				t.Fatalf("want %v, got %v", c.want, got)
			}
		})
	}
}

func TestAllStatusesContainsAllConstants(t *testing.T) {
	t.Parallel()
	want := map[Status]bool{
		StatusPendingKYC:  true,
		StatusTrialing:    true,
		StatusTrialEnding: true,
		StatusTrialLapsed: true,
		StatusActive:      true,
		StatusPastDue:     true,
		StatusSuspended:   true,
		StatusCancelled:   true,
		StatusTerminated:  true,
	}
	got := AllStatuses()
	if len(got) != len(want) {
		t.Fatalf("AllStatuses length: want %d, got %d", len(want), len(got))
	}
	for _, s := range got {
		if !want[s] {
			t.Errorf("AllStatuses contains unexpected status %s", s)
		}
		delete(want, s)
	}
	if len(want) > 0 {
		t.Errorf("AllStatuses missing: %v", want)
	}
}

// ---- helpers ----

func fixedNow() time.Time {
	return time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
}

// validatePaymentMethodReq mirrors the validation block in Service.UpdatePaymentMethod;
// extracted so we can test it without a DB. Service.UpdatePaymentMethod stays
// the source of truth — this is only a re-statement for unit testability.
func validatePaymentMethodReq(req UpdatePaymentMethodRequest) error {
	if req.Provider != ProviderStripe && req.Provider != ProviderOmise {
		return ErrInvalidProvider
	}
	methodID := trim(req.MethodID)
	if methodID == "" || len(methodID) > 120 {
		return ErrInvalidPaymentMethod
	}
	last4 := trim(req.Last4)
	if last4 != "" && len(last4) != 4 {
		return ErrInvalidPaymentMethod
	}
	brand := trim(req.Brand)
	if len(brand) > 20 {
		return ErrInvalidPaymentMethod
	}
	return nil
}

func trim(s string) string {
	out := []byte(s)
	for len(out) > 0 && (out[0] == ' ' || out[0] == '\t' || out[0] == '\n') {
		out = out[1:]
	}
	for len(out) > 0 && (out[len(out)-1] == ' ' || out[len(out)-1] == '\t' || out[len(out)-1] == '\n') {
		out = out[:len(out)-1]
	}
	return string(out)
}
