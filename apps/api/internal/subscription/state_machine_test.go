package subscription

import "testing"

func TestCanTransition(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		from Status
		to   Status
		want bool
	}{
		// --- legal transitions (must mirror legalTransitions slice) ---
		{"pending_kyc → trialing", StatusPendingKYC, StatusTrialing, true},

		{"trialing → trial_ending", StatusTrialing, StatusTrialEnding, true},
		{"trialing → cancelled", StatusTrialing, StatusCancelled, true},

		{"trial_ending → active", StatusTrialEnding, StatusActive, true},
		{"trial_ending → trial_lapsed", StatusTrialEnding, StatusTrialLapsed, true},

		{"trial_lapsed → active", StatusTrialLapsed, StatusActive, true},
		{"trial_lapsed → suspended", StatusTrialLapsed, StatusSuspended, true},

		{"active → past_due", StatusActive, StatusPastDue, true},

		{"past_due → active", StatusPastDue, StatusActive, true},
		{"past_due → suspended", StatusPastDue, StatusSuspended, true},

		{"suspended → active", StatusSuspended, StatusActive, true},
		{"suspended → cancelled", StatusSuspended, StatusCancelled, true},

		{"cancelled → terminated", StatusCancelled, StatusTerminated, true},
		{"cancelled → trialing", StatusCancelled, StatusTrialing, true},

		// --- illegal transitions ---
		// Identity transitions are not allowed (idempotency is service-layer).
		{"pending_kyc → pending_kyc", StatusPendingKYC, StatusPendingKYC, false},
		{"active → active", StatusActive, StatusActive, false},
		{"trialing → trialing", StatusTrialing, StatusTrialing, false},

		// pending_kyc cannot jump directly to most states.
		{"pending_kyc → active", StatusPendingKYC, StatusActive, false},
		{"pending_kyc → trial_ending", StatusPendingKYC, StatusTrialEnding, false},
		{"pending_kyc → cancelled", StatusPendingKYC, StatusCancelled, false},
		{"pending_kyc → suspended", StatusPendingKYC, StatusSuspended, false},
		{"pending_kyc → terminated", StatusPendingKYC, StatusTerminated, false},

		// trialing cannot skip trial_ending.
		{"trialing → active", StatusTrialing, StatusActive, false},
		{"trialing → trial_lapsed", StatusTrialing, StatusTrialLapsed, false},
		{"trialing → suspended", StatusTrialing, StatusSuspended, false},
		{"trialing → terminated", StatusTrialing, StatusTerminated, false},
		{"trialing → past_due", StatusTrialing, StatusPastDue, false},

		// trial_ending → only active or trial_lapsed.
		{"trial_ending → cancelled", StatusTrialEnding, StatusCancelled, false},
		{"trial_ending → suspended", StatusTrialEnding, StatusSuspended, false},
		{"trial_ending → past_due", StatusTrialEnding, StatusPastDue, false},
		{"trial_ending → pending_kyc", StatusTrialEnding, StatusPendingKYC, false},

		// trial_lapsed → only active or suspended.
		{"trial_lapsed → cancelled", StatusTrialLapsed, StatusCancelled, false},
		{"trial_lapsed → trialing", StatusTrialLapsed, StatusTrialing, false},
		{"trial_lapsed → terminated", StatusTrialLapsed, StatusTerminated, false},

		// active → only past_due (Phase 1 — no direct cancel from active).
		{"active → cancelled", StatusActive, StatusCancelled, false},
		{"active → suspended", StatusActive, StatusSuspended, false},
		{"active → terminated", StatusActive, StatusTerminated, false},
		{"active → trialing", StatusActive, StatusTrialing, false},

		// past_due → only active or suspended.
		{"past_due → cancelled", StatusPastDue, StatusCancelled, false},
		{"past_due → terminated", StatusPastDue, StatusTerminated, false},

		// suspended → only active or cancelled.
		{"suspended → terminated", StatusSuspended, StatusTerminated, false},
		{"suspended → trialing", StatusSuspended, StatusTrialing, false},
		{"suspended → past_due", StatusSuspended, StatusPastDue, false},

		// cancelled → only terminated or trialing.
		{"cancelled → active", StatusCancelled, StatusActive, false},
		{"cancelled → suspended", StatusCancelled, StatusSuspended, false},
		{"cancelled → past_due", StatusCancelled, StatusPastDue, false},

		// terminated is terminal — no outbound edges.
		{"terminated → active", StatusTerminated, StatusActive, false},
		{"terminated → trialing", StatusTerminated, StatusTrialing, false},
		{"terminated → cancelled", StatusTerminated, StatusCancelled, false},
		{"terminated → pending_kyc", StatusTerminated, StatusPendingKYC, false},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got := CanTransition(c.from, c.to)
			if got != c.want {
				t.Fatalf("CanTransition(%s, %s) = %v, want %v", c.from, c.to, got, c.want)
			}
		})
	}
}

// TestStateMachineExhaustive verifies every (from, to) pair across all known
// statuses against the legalTransitions slice — guarantees the table-driven
// test above stays in sync with the diagram.
func TestStateMachineExhaustive(t *testing.T) {
	t.Parallel()
	legal := make(map[StateTransition]struct{}, len(legalTransitions))
	for _, tr := range legalTransitions {
		legal[tr] = struct{}{}
	}
	for _, from := range AllStatuses() {
		for _, to := range AllStatuses() {
			_, want := legal[StateTransition{From: from, To: to}]
			got := CanTransition(from, to)
			if got != want {
				t.Errorf("CanTransition(%s, %s) = %v, want %v", from, to, got, want)
			}
		}
	}
}

func TestLegalTransitionsReturnsCopy(t *testing.T) {
	t.Parallel()
	a := LegalTransitions()
	a[0] = StateTransition{From: StatusTerminated, To: StatusPendingKYC} // mutate copy
	b := LegalTransitions()
	if b[0] == (StateTransition{From: StatusTerminated, To: StatusPendingKYC}) {
		t.Fatalf("LegalTransitions must return a defensive copy; mutation leaked")
	}
}

func TestTerminatedIsTerminal(t *testing.T) {
	t.Parallel()
	for _, to := range AllStatuses() {
		if CanTransition(StatusTerminated, to) {
			t.Errorf("terminated should have no outbound edges, got terminated → %s", to)
		}
	}
}
