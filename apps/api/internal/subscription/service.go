package subscription

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
)

// trialDuration is the free-tier length per plan.md §1 ("ฟรี 365 วันแรก").
const trialDuration = 365 * 24 * time.Hour

type Service struct {
	repo *Repository
	// now is injectable for deterministic tests (e.g. duration math).
	now func() time.Time
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo, now: time.Now}
}

// CreateForAccount initialises a subscription in `pending_kyc` status.
// Called once at signup. NOT idempotent — returns ErrAlreadyExists on dup.
// Prefer EnsureForAccount if you can't guarantee single-call semantics.
func (s *Service) CreateForAccount(ctx context.Context, accountID uuid.UUID) (*Subscription, error) {
	if accountID == uuid.Nil {
		return nil, ErrInvalidRequest
	}
	return s.repo.Create(ctx, accountID)
}

// EnsureForAccount is the idempotent variant — safe to call from signup or as a
// worker safety-net. Returns the existing sub on conflict.
func (s *Service) EnsureForAccount(ctx context.Context, accountID uuid.UUID) (*Subscription, error) {
	if accountID == uuid.Nil {
		return nil, ErrInvalidRequest
	}
	return s.repo.Ensure(ctx, accountID)
}

// Get returns the subscription for an account.
func (s *Service) Get(ctx context.Context, accountID uuid.UUID) (*Subscription, error) {
	return s.repo.GetByAccount(ctx, accountID)
}

// ActivateTrial transitions pending_kyc → trialing and starts the 365-day clock.
// Idempotent: if the subscription is already in trialing (or beyond), it returns
// the current row without error — useful when first-booking + admin-grant races.
func (s *Service) ActivateTrial(ctx context.Context, accountID uuid.UUID, source string) (*Subscription, error) {
	source = strings.TrimSpace(source)
	if source == "" {
		source = "first_booking"
	}

	current, err := s.repo.GetByAccount(ctx, accountID)
	if err != nil {
		return nil, err
	}
	// Idempotency: trial already started (any post-pending_kyc state) → no-op.
	if current.Status != StatusPendingKYC {
		return current, nil
	}

	now := s.now()
	trialEnds := now.Add(trialDuration)
	return s.repo.Transition(ctx, accountID,
		StatusPendingKYC, StatusTrialing,
		"trial_activated", "system", nil,
		TransitionUpdate{
			TrialStartedAt: &now,
			TrialEndsAt:    &trialEnds,
		},
		map[string]any{"source": source},
	)
}

// MarkTrialEnding: trialing → trial_ending (worker, 30d before trial_ends_at).
// Idempotent for already-ended states (returns current row).
func (s *Service) MarkTrialEnding(ctx context.Context, accountID uuid.UUID) (*Subscription, error) {
	return s.workerTransition(ctx, accountID, StatusTrialing, StatusTrialEnding, "trial_ending", nil)
}

// MarkTrialLapsed: trial_ending → trial_lapsed (worker, trial expired w/o pmt).
func (s *Service) MarkTrialLapsed(ctx context.Context, accountID uuid.UUID) (*Subscription, error) {
	return s.workerTransition(ctx, accountID, StatusTrialEnding, StatusTrialLapsed, "trial_lapsed", nil)
}

// Suspend: trial_lapsed → suspended (worker, after 14d grace).
// Also supported: past_due → suspended (final retry fail) but callers from the
// worker only hit the trial_lapsed branch.
func (s *Service) Suspend(ctx context.Context, accountID uuid.UUID) (*Subscription, error) {
	now := s.now()
	current, err := s.repo.GetByAccount(ctx, accountID)
	if err != nil {
		return nil, err
	}
	// Idempotent if already suspended/cancelled/terminated.
	if current.Status == StatusSuspended || current.Status == StatusCancelled || current.Status == StatusTerminated {
		return current, nil
	}
	// Both trial_lapsed and past_due → suspended are legal; pick the one
	// matching current state.
	from := current.Status
	if !CanTransition(from, StatusSuspended) {
		return nil, ErrInvalidTransition
	}
	return s.repo.Transition(ctx, accountID,
		from, StatusSuspended,
		"suspended", "system", nil,
		TransitionUpdate{SuspendedAt: &now},
		nil,
	)
}

// Reactivate: suspended → active (manual admin action). Not exposed in handler
// in Phase 1 — service-only entry point.
func (s *Service) Reactivate(ctx context.Context, accountID uuid.UUID, actorID *uuid.UUID) (*Subscription, error) {
	return s.repo.Transition(ctx, accountID,
		StatusSuspended, StatusActive,
		"reactivated", "platform_admin", actorID,
		TransitionUpdate{},
		nil,
	)
}

// Cancel: owner-initiated cancellation. Legal from trialing or suspended;
// from any other state we return ErrInvalidTransition.
func (s *Service) Cancel(ctx context.Context, accountID uuid.UUID, actorID *uuid.UUID, reason string) (*Subscription, error) {
	reason = strings.TrimSpace(reason)
	now := s.now()

	current, err := s.repo.GetByAccount(ctx, accountID)
	if err != nil {
		return nil, err
	}
	// Idempotency on terminal-ish states.
	if current.Status == StatusCancelled || current.Status == StatusTerminated {
		return current, nil
	}
	if !CanTransition(current.Status, StatusCancelled) {
		return nil, ErrInvalidTransition
	}

	actorType := "hotel_staff"
	if actorID == nil {
		actorType = "system"
	}
	patch := TransitionUpdate{CancelledAt: &now}
	if reason != "" {
		patch.CancellationReason = &reason
	}
	return s.repo.Transition(ctx, accountID,
		current.Status, StatusCancelled,
		"cancelled", actorType, actorID,
		patch,
		map[string]any{"reason": reason},
	)
}

// Terminate: cancelled → terminated (worker, after 90d retention).
func (s *Service) Terminate(ctx context.Context, accountID uuid.UUID) (*Subscription, error) {
	return s.workerTransition(ctx, accountID, StatusCancelled, StatusTerminated, "terminated", nil)
}

// UpdatePaymentMethod stores a gateway-returned token + display attrs.
// Owner-only at the handler layer; service-side just validates input.
func (s *Service) UpdatePaymentMethod(
	ctx context.Context,
	accountID uuid.UUID,
	actorID *uuid.UUID,
	req UpdatePaymentMethodRequest,
) (*Subscription, error) {
	if req.Provider != ProviderStripe && req.Provider != ProviderOmise {
		return nil, ErrInvalidProvider
	}
	methodID := strings.TrimSpace(req.MethodID)
	if methodID == "" || len(methodID) > 120 {
		return nil, ErrInvalidPaymentMethod
	}
	last4 := strings.TrimSpace(req.Last4)
	if last4 != "" && len(last4) != 4 {
		return nil, ErrInvalidPaymentMethod
	}
	brand := strings.TrimSpace(req.Brand)
	if len(brand) > 20 {
		return nil, ErrInvalidPaymentMethod
	}
	return s.repo.UpdatePaymentMethod(ctx, accountID, actorID, req.Provider, methodID, last4, brand)
}

// RunMaintenance is invoked by the worker on a 60s tick. Each step is
// best-effort: failures are counted and logged but do not stop the run.
//
// Steps mirror plan.md §6.5 timing:
//   1. trialing  → trial_ending  (30d before trial_ends_at)
//   2. trial_ending → trial_lapsed (trial expired, no payment method)
//   3. trial_lapsed → suspended  (14d grace expired)
//   4. suspended → cancelled     (30d in suspended)
//   5. cancelled → terminated    (90d retention)
func (s *Service) RunMaintenance(ctx context.Context) MaintenanceSummary {
	var sum MaintenanceSummary
	logger := slog.With("component", "subscription.maintenance")

	step := func(label string, fn func(context.Context) ([]uuid.UUID, error), apply func(context.Context, uuid.UUID) (*Subscription, error), counter *int) {
		ids, err := fn(ctx)
		if err != nil {
			logger.Error("scan", "step", label, "err", err)
			sum.ErrorCount++
			return
		}
		for _, id := range ids {
			if ctx.Err() != nil {
				return
			}
			if _, err := apply(ctx, id); err != nil {
				// Race-tolerant: the row may have moved already (e.g. owner
				// just cancelled). ErrInvalidTransition is expected.
				if errors.Is(err, ErrInvalidTransition) || errors.Is(err, ErrSubscriptionNotFound) {
					continue
				}
				logger.Error("apply", "step", label, "account_id", id, "err", err)
				sum.ErrorCount++
				continue
			}
			*counter++
		}
	}

	step("trial_ending", s.repo.FindTrialEndingCandidates,
		func(ctx context.Context, id uuid.UUID) (*Subscription, error) { return s.MarkTrialEnding(ctx, id) },
		&sum.TrialEnding)
	step("trial_lapsed", s.repo.FindTrialLapsedCandidates,
		func(ctx context.Context, id uuid.UUID) (*Subscription, error) { return s.MarkTrialLapsed(ctx, id) },
		&sum.TrialLapsed)
	step("suspended", s.repo.FindSuspendCandidates,
		func(ctx context.Context, id uuid.UUID) (*Subscription, error) { return s.Suspend(ctx, id) },
		&sum.Suspended)
	step("cancelled", s.repo.FindCancelCandidates,
		func(ctx context.Context, id uuid.UUID) (*Subscription, error) {
			return s.Cancel(ctx, id, nil, "auto-cancelled after 30d suspension")
		},
		&sum.Cancelled)
	step("terminated", s.repo.FindTerminateCandidates,
		func(ctx context.Context, id uuid.UUID) (*Subscription, error) { return s.Terminate(ctx, id) },
		&sum.Terminated)

	return sum
}

// workerTransition is a small wrapper for worker-driven transitions: it
// short-circuits when the subscription is already past the `from` state, so
// the loop tolerates concurrent writes (e.g. owner cancels mid-scan).
func (s *Service) workerTransition(ctx context.Context, accountID uuid.UUID, from, to Status, event string, payload map[string]any) (*Subscription, error) {
	current, err := s.repo.GetByAccount(ctx, accountID)
	if err != nil {
		return nil, err
	}
	if current.Status != from {
		return current, nil
	}
	return s.repo.Transition(ctx, accountID, from, to, event, "system", nil, TransitionUpdate{}, payload)
}
