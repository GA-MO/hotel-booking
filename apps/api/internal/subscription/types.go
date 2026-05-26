package subscription

import (
	"time"

	"github.com/google/uuid"
)

// Status is the typed enum mirroring the CHECK constraint on subscriptions.status.
type Status string

const (
	StatusPendingKYC   Status = "pending_kyc"
	StatusTrialing     Status = "trialing"
	StatusTrialEnding  Status = "trial_ending"
	StatusTrialLapsed  Status = "trial_lapsed"
	StatusActive       Status = "active"
	StatusPastDue      Status = "past_due"
	StatusSuspended    Status = "suspended"
	StatusCancelled    Status = "cancelled"
	StatusTerminated   Status = "terminated"
)

// AllStatuses returns every legal status (used by exhaustive tests).
func AllStatuses() []Status {
	return []Status{
		StatusPendingKYC, StatusTrialing, StatusTrialEnding, StatusTrialLapsed,
		StatusActive, StatusPastDue, StatusSuspended, StatusCancelled, StatusTerminated,
	}
}

// PaymentProvider mirrors the CHECK constraint on subscriptions.payment_provider.
type PaymentProvider string

const (
	ProviderStripe PaymentProvider = "stripe"
	ProviderOmise  PaymentProvider = "omise"
)

// StateTransition describes a legal state transition. The list in state_machine.go
// is the single source of truth — the repository and worker reference it indirectly
// via CanTransition.
type StateTransition struct {
	From Status
	To   Status
}

// Subscription is the row from `subscriptions`. Pointer fields are nullable in DB.
type Subscription struct {
	ID        uuid.UUID `json:"id"`
	AccountID uuid.UUID `json:"account_id"`

	Status       Status  `json:"status"`
	PlanCode     string  `json:"plan_code,omitempty"`
	BillingCycle string  `json:"billing_cycle,omitempty"`

	TrialStartedAt *time.Time `json:"trial_started_at,omitempty"`
	TrialEndsAt    *time.Time `json:"trial_ends_at,omitempty"`

	CurrentPeriodStart *time.Time `json:"current_period_start,omitempty"`
	CurrentPeriodEnd   *time.Time `json:"current_period_end,omitempty"`

	RoomCountSnapshot *int    `json:"room_count_snapshot,omitempty"`
	UnitPriceCents    *int64  `json:"unit_price_cents,omitempty"`
	Currency          string  `json:"currency,omitempty"`

	PaymentProvider    string `json:"payment_provider,omitempty"`
	PaymentMethodID    string `json:"payment_method_id,omitempty"`
	PaymentMethodLast4 string `json:"payment_method_last4,omitempty"`
	PaymentMethodBrand string `json:"payment_method_brand,omitempty"`

	CancelledAt        *time.Time `json:"cancelled_at,omitempty"`
	CancellationReason string     `json:"cancellation_reason,omitempty"`
	SuspendedAt        *time.Time `json:"suspended_at,omitempty"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// HasPaymentMethod returns true when a payment method token has been captured.
// Used by the state machine ("trial_ending → active" gate).
func (s *Subscription) HasPaymentMethod() bool {
	return s != nil && s.PaymentMethodID != ""
}

// ----- request DTOs -----

// CreateRequest is the (internal) input for creating a new subscription row.
// External callers should use Service.CreateForAccount / EnsureForAccount.
type CreateRequest struct {
	AccountID uuid.UUID
}

// ActivateTrialRequest captures *why* the trial clock started — for the audit
// log. Source values: "first_booking" | "admin_grant" | "max_window".
type ActivateTrialRequest struct {
	Source string `json:"source"`
}

// UpdatePaymentMethodRequest is the body for POST /v1/subscription/payment-method.
// The gateway client SDK (Stripe.js / Omise.js) returns a token-like method_id;
// we store it verbatim plus a few display attributes. NO server-side gateway call.
type UpdatePaymentMethodRequest struct {
	Provider PaymentProvider `json:"provider"`
	MethodID string          `json:"method_id"`
	Last4    string          `json:"last4,omitempty"`
	Brand    string          `json:"brand,omitempty"`
}

// CancelRequest is the body for POST /v1/subscription/cancel.
type CancelRequest struct {
	Reason string `json:"reason,omitempty"`
}

// MaintenanceSummary is the counter set returned by Service.RunMaintenance,
// useful for worker logging.
type MaintenanceSummary struct {
	TrialEnding  int `json:"trial_ending"`
	TrialLapsed  int `json:"trial_lapsed"`
	Suspended    int `json:"suspended"`
	Cancelled    int `json:"cancelled"`
	Terminated   int `json:"terminated"`
	ErrorCount   int `json:"error_count"`
}
