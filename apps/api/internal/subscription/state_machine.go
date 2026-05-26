package subscription

// legalTransitions is the source-of-truth state diagram from plan.md §6.5.
//
//   pending_kyc  → trialing                       (signup completes / first booking)
//   trialing     → trial_ending                   (system: 30d before trial_ends_at)
//   trialing     → cancelled                      (owner explicit cancel)
//   trial_ending → active                         (payment method captured + trial expires)
//   trial_ending → trial_lapsed                   (trial expired, no payment method)
//   trial_lapsed → active                         (payment captured during 14d grace)
//   trial_lapsed → suspended                      (grace expired)
//   active       → past_due                       (charge fail)
//   past_due     → active                         (charge retry succeeds)
//   past_due     → suspended                      (final retry fails)
//   suspended    → active                         (manual reactivation)
//   suspended    → cancelled                      (auto after 30d)
//   cancelled    → terminated                     (after 90d data-retention)
//   cancelled    → trialing                       (re-subscribe within grace — TBD)
//
// Terminated is the terminal state — no outbound edges.
var legalTransitions = []StateTransition{
	{StatusPendingKYC, StatusTrialing},

	{StatusTrialing, StatusTrialEnding},
	{StatusTrialing, StatusCancelled},

	{StatusTrialEnding, StatusActive},
	{StatusTrialEnding, StatusTrialLapsed},

	{StatusTrialLapsed, StatusActive},
	{StatusTrialLapsed, StatusSuspended},

	{StatusActive, StatusPastDue},

	{StatusPastDue, StatusActive},
	{StatusPastDue, StatusSuspended},

	{StatusSuspended, StatusActive},
	{StatusSuspended, StatusCancelled},

	{StatusCancelled, StatusTerminated},
	{StatusCancelled, StatusTrialing},
}

// transitionSet is built once for O(1) lookup. We avoid building it inside
// CanTransition on every call to keep that path branch-free.
var transitionSet = func() map[StateTransition]struct{} {
	m := make(map[StateTransition]struct{}, len(legalTransitions))
	for _, t := range legalTransitions {
		m[t] = struct{}{}
	}
	return m
}()

// CanTransition reports whether moving from -> to is allowed by the state diagram.
// Identity transitions (from == to) are NOT allowed here; idempotency is handled
// at the service layer (e.g. ActivateTrial is a no-op if already trialing).
func CanTransition(from, to Status) bool {
	_, ok := transitionSet[StateTransition{From: from, To: to}]
	return ok
}

// LegalTransitions returns a defensive copy of the transition table (read-only
// helper for tests and admin tooling).
func LegalTransitions() []StateTransition {
	out := make([]StateTransition, len(legalTransitions))
	copy(out, legalTransitions)
	return out
}
