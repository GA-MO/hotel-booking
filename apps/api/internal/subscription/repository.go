package subscription

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// selectColumns is the canonical column list — kept identical between INSERT
// RETURNING and SELECT so scanSubscription can be shared.
const selectColumns = `
	id, account_id, status,
	COALESCE(plan_code, ''), COALESCE(billing_cycle, ''),
	trial_started_at, trial_ends_at,
	current_period_start, current_period_end,
	room_count_snapshot, unit_price_cents, COALESCE(currency, ''),
	COALESCE(payment_provider, ''),
	COALESCE(payment_method_id, ''),
	COALESCE(payment_method_last4, ''),
	COALESCE(payment_method_brand, ''),
	cancelled_at, COALESCE(cancellation_reason, ''),
	suspended_at,
	created_at, updated_at
`

func scanSubscription(row pgx.Row) (*Subscription, error) {
	var s Subscription
	if err := row.Scan(
		&s.ID, &s.AccountID, &s.Status,
		&s.PlanCode, &s.BillingCycle,
		&s.TrialStartedAt, &s.TrialEndsAt,
		&s.CurrentPeriodStart, &s.CurrentPeriodEnd,
		&s.RoomCountSnapshot, &s.UnitPriceCents, &s.Currency,
		&s.PaymentProvider,
		&s.PaymentMethodID, &s.PaymentMethodLast4, &s.PaymentMethodBrand,
		&s.CancelledAt, &s.CancellationReason,
		&s.SuspendedAt,
		&s.CreatedAt, &s.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &s, nil
}

// Create inserts a new row in `pending_kyc` state and writes a `created` event.
// Returns ErrAlreadyExists on UNIQUE(account_id) violation.
func (r *Repository) Create(ctx context.Context, accountID uuid.UUID) (*Subscription, error) {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	row := tx.QueryRow(ctx, `
		INSERT INTO subscriptions (account_id, status)
		VALUES ($1, $2)
		RETURNING `+selectColumns,
		accountID, string(StatusPendingKYC),
	)
	sub, err := scanSubscription(row)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrAlreadyExists
		}
		return nil, fmt.Errorf("insert subscription: %w", err)
	}
	if err := insertEvent(ctx, tx, sub.ID, "created", "system", nil, map[string]any{
		"initial_status": string(StatusPendingKYC),
	}); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}
	return sub, nil
}

// Ensure is the idempotent variant of Create — uses ON CONFLICT DO NOTHING so
// it's safe to call from signup flow or worker safety-net. Returns the existing
// row when there's a conflict.
func (r *Repository) Ensure(ctx context.Context, accountID uuid.UUID) (*Subscription, error) {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	// Try to insert; on conflict (account_id) skip and select the existing row.
	row := tx.QueryRow(ctx, `
		WITH ins AS (
			INSERT INTO subscriptions (account_id, status)
			VALUES ($1, $2)
			ON CONFLICT (account_id) DO NOTHING
			RETURNING `+selectColumns+`
		)
		SELECT * FROM ins
		UNION ALL
		SELECT `+selectColumns+` FROM subscriptions
		WHERE account_id = $1
		LIMIT 1
	`, accountID, string(StatusPendingKYC))

	sub, err := scanSubscription(row)
	if err != nil {
		return nil, fmt.Errorf("ensure subscription: %w", err)
	}

	// Only record the audit event when the row was just created (status still
	// the initial pending_kyc AND no prior events for this sub_id). We check
	// the latter to keep Ensure cheap and avoid double-logging on race.
	var hasEvent bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM subscription_events WHERE subscription_id = $1)
	`, sub.ID).Scan(&hasEvent); err != nil {
		return nil, fmt.Errorf("check events: %w", err)
	}
	if !hasEvent {
		if err := insertEvent(ctx, tx, sub.ID, "created", "system", nil, map[string]any{
			"initial_status": string(StatusPendingKYC),
			"via":            "ensure",
		}); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}
	return sub, nil
}

// GetByAccount fetches the (single) subscription for an account.
func (r *Repository) GetByAccount(ctx context.Context, accountID uuid.UUID) (*Subscription, error) {
	row := r.db.QueryRow(ctx, `
		SELECT `+selectColumns+`
		FROM subscriptions
		WHERE account_id = $1
	`, accountID)
	sub, err := scanSubscription(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrSubscriptionNotFound
		}
		return nil, err
	}
	return sub, nil
}

// TransitionUpdate is the set of patches applied inside a state transition.
// Nil pointer fields are left unchanged.
type TransitionUpdate struct {
	TrialStartedAt     *time.Time
	TrialEndsAt        *time.Time
	CurrentPeriodStart *time.Time
	CurrentPeriodEnd   *time.Time
	CancelledAt        *time.Time
	CancellationReason *string
	SuspendedAt        *time.Time
}

// Transition atomically:
//  1. SELECT FOR UPDATE the subscription (locks the row)
//  2. validates from -> to via CanTransition
//  3. UPDATEs status (+ any non-nil columns in patch)
//  4. INSERTs a row in subscription_events
//
// Returns ErrInvalidTransition if the current status doesn't match `from`
// or the transition is not legal. Returns ErrSubscriptionNotFound if the
// account has no subscription.
func (r *Repository) Transition(
	ctx context.Context,
	accountID uuid.UUID,
	from, to Status,
	eventType string,
	actorType string,
	actorID *uuid.UUID,
	patch TransitionUpdate,
	eventPayload map[string]any,
) (*Subscription, error) {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var subID uuid.UUID
	var currentStatus Status
	if err := tx.QueryRow(ctx, `
		SELECT id, status FROM subscriptions
		WHERE account_id = $1
		FOR UPDATE
	`, accountID).Scan(&subID, &currentStatus); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrSubscriptionNotFound
		}
		return nil, fmt.Errorf("lock subscription: %w", err)
	}

	if currentStatus != from {
		return nil, fmt.Errorf("%w: have %s, expected %s", ErrInvalidTransition, currentStatus, from)
	}
	if !CanTransition(from, to) {
		return nil, fmt.Errorf("%w: %s → %s", ErrInvalidTransition, from, to)
	}

	row := tx.QueryRow(ctx, `
		UPDATE subscriptions SET
		  status               = $2,
		  trial_started_at     = COALESCE($3, trial_started_at),
		  trial_ends_at        = COALESCE($4, trial_ends_at),
		  current_period_start = COALESCE($5, current_period_start),
		  current_period_end   = COALESCE($6, current_period_end),
		  cancelled_at         = COALESCE($7, cancelled_at),
		  cancellation_reason  = COALESCE($8, cancellation_reason),
		  suspended_at         = COALESCE($9, suspended_at)
		WHERE id = $1
		RETURNING `+selectColumns,
		subID, string(to),
		ptrOrNil(patch.TrialStartedAt),
		ptrOrNil(patch.TrialEndsAt),
		ptrOrNil(patch.CurrentPeriodStart),
		ptrOrNil(patch.CurrentPeriodEnd),
		ptrOrNil(patch.CancelledAt),
		ptrOrNil(patch.CancellationReason),
		ptrOrNil(patch.SuspendedAt),
	)
	sub, err := scanSubscription(row)
	if err != nil {
		return nil, fmt.Errorf("update subscription: %w", err)
	}

	if eventPayload == nil {
		eventPayload = map[string]any{}
	}
	eventPayload["from"] = string(from)
	eventPayload["to"] = string(to)
	if err := insertEvent(ctx, tx, subID, eventType, actorType, actorID, eventPayload); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}
	return sub, nil
}

// UpdatePaymentMethod writes the gateway-returned token + display attrs.
// No state transition — this just attaches metadata. Records an audit event.
func (r *Repository) UpdatePaymentMethod(
	ctx context.Context,
	accountID uuid.UUID,
	actorID *uuid.UUID,
	provider PaymentProvider,
	methodID, last4, brand string,
) (*Subscription, error) {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var subID uuid.UUID
	if err := tx.QueryRow(ctx, `
		SELECT id FROM subscriptions WHERE account_id = $1 FOR UPDATE
	`, accountID).Scan(&subID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrSubscriptionNotFound
		}
		return nil, fmt.Errorf("lock subscription: %w", err)
	}

	row := tx.QueryRow(ctx, `
		UPDATE subscriptions SET
		  payment_provider     = $2,
		  payment_method_id    = $3,
		  payment_method_last4 = NULLIF($4, ''),
		  payment_method_brand = NULLIF($5, '')
		WHERE id = $1
		RETURNING `+selectColumns,
		subID, string(provider), methodID, last4, brand,
	)
	sub, err := scanSubscription(row)
	if err != nil {
		return nil, fmt.Errorf("update payment method: %w", err)
	}

	// Don't log the full method_id in the audit payload (it's a token, not a
	// secret, but Less Stored Is More Better when we don't need it).
	if err := insertEvent(ctx, tx, subID, "payment_method_updated", "hotel_staff", actorID, map[string]any{
		"provider": string(provider),
		"last4":    last4,
		"brand":    brand,
	}); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}
	return sub, nil
}

// ----- worker-scan queries -----
//
// The worker's RunMaintenance flow needs to find accounts ready for each
// automatic transition. These return account_ids (not full rows) so the
// service can re-run Transition with proper row locking.

func (r *Repository) accountIDsByQuery(ctx context.Context, query string, args ...any) ([]uuid.UUID, error) {
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// FindTrialEndingCandidates: status='trialing' AND trial_ends_at < NOW() + 30 days.
func (r *Repository) FindTrialEndingCandidates(ctx context.Context) ([]uuid.UUID, error) {
	return r.accountIDsByQuery(ctx, `
		SELECT account_id FROM subscriptions
		WHERE status = 'trialing'
		  AND trial_ends_at IS NOT NULL
		  AND trial_ends_at < NOW() + INTERVAL '30 days'
	`)
}

// FindTrialLapsedCandidates: status='trial_ending' AND trial_ends_at < NOW()
// AND payment_method_id IS NULL.
func (r *Repository) FindTrialLapsedCandidates(ctx context.Context) ([]uuid.UUID, error) {
	return r.accountIDsByQuery(ctx, `
		SELECT account_id FROM subscriptions
		WHERE status = 'trial_ending'
		  AND trial_ends_at IS NOT NULL
		  AND trial_ends_at < NOW()
		  AND payment_method_id IS NULL
	`)
}

// FindSuspendCandidates: status='trial_lapsed' AND updated_at < NOW() - 14 days.
func (r *Repository) FindSuspendCandidates(ctx context.Context) ([]uuid.UUID, error) {
	return r.accountIDsByQuery(ctx, `
		SELECT account_id FROM subscriptions
		WHERE status = 'trial_lapsed'
		  AND updated_at < NOW() - INTERVAL '14 days'
	`)
}

// FindCancelCandidates: status='suspended' AND suspended_at < NOW() - 30 days.
func (r *Repository) FindCancelCandidates(ctx context.Context) ([]uuid.UUID, error) {
	return r.accountIDsByQuery(ctx, `
		SELECT account_id FROM subscriptions
		WHERE status = 'suspended'
		  AND suspended_at IS NOT NULL
		  AND suspended_at < NOW() - INTERVAL '30 days'
	`)
}

// FindTerminateCandidates: status='cancelled' AND cancelled_at < NOW() - 90 days.
func (r *Repository) FindTerminateCandidates(ctx context.Context) ([]uuid.UUID, error) {
	return r.accountIDsByQuery(ctx, `
		SELECT account_id FROM subscriptions
		WHERE status = 'cancelled'
		  AND cancelled_at IS NOT NULL
		  AND cancelled_at < NOW() - INTERVAL '90 days'
	`)
}

// ----- helpers -----

func insertEvent(
	ctx context.Context,
	tx pgx.Tx,
	subscriptionID uuid.UUID,
	eventType, actorType string,
	actorID *uuid.UUID,
	payload map[string]any,
) error {
	if payload == nil {
		payload = map[string]any{}
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal event payload: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO subscription_events (subscription_id, event_type, actor_type, actor_id, payload)
		VALUES ($1, $2, NULLIF($3, ''), $4, $5)
	`, subscriptionID, eventType, actorType, actorID, raw); err != nil {
		return fmt.Errorf("insert event: %w", err)
	}
	return nil
}

func ptrOrNil[T any](p *T) any {
	if p == nil {
		return nil
	}
	return *p
}

func isUniqueViolation(err error) bool {
	const code = "23505"
	type pgErr interface{ SQLState() string }
	var pe pgErr
	if errors.As(err, &pe) {
		return pe.SQLState() == code
	}
	return false
}
