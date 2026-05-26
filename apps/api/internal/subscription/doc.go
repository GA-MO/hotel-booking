// Package subscription tracks the per-account billing state machine.
// Phase 1 is state-only — no Stripe/Omise SDK calls yet; payment method
// metadata is captured for future use (final tier prices TBD pending
// customer research; see plan.md §12).
//
// State machine:
//
//   pending_kyc → trialing
//   trialing → trial_ending     (system, 30 days before trial_ends_at)
//   trialing → cancelled        (owner-initiated)
//   trial_ending → active       (payment method captured)
//   trial_ending → trial_lapsed (trial expired, no payment method)
//   trial_lapsed → active       (payment captured during grace)
//   trial_lapsed → suspended    (14-day grace expired)
//   active → past_due           (charge failed; not yet wired)
//   past_due → active           (retry succeeded; not yet wired)
//   past_due → suspended        (final retry failed)
//   suspended → active          (manual reactivation)
//   suspended → cancelled       (auto after 30 days)
//   cancelled → terminated      (auto after 90 days — data retention)
//
// All transitions go through Service methods and record an append-only
// row in `subscription_events`. [Service.RunMaintenance], called from
// the worker every ~60s, advances time-based transitions automatically
// and is race-tolerant (ErrInvalidTransition is silently skipped — a
// concurrent owner-cancel can race the worker without erroring).
//
// [Service.EnsureForAccount] is idempotent (CTE + ON CONFLICT) and is
// invoked from auth's signup hook so a row exists immediately for every
// new account. Also safe to call as a worker safety-net backfill for
// legacy accounts.
//
// Trial clock starts at the first confirmed booking via
// [Service.ActivateTrial] (capped at 60 days post-signup as a safeguard
// against indefinitely-paused accounts), giving free time only when the
// hotel actively uses the system.
package subscription
