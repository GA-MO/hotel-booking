package subscription

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Integration tests — require a Postgres reachable via TEST_DATABASE_URL with
// all migrations applied. Skip if not set.

func setupTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set TEST_DATABASE_URL to run integration tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("ping: %v", err)
	}
	return pool
}

// seedAccount inserts a throwaway account row and registers cleanup. The
// subscription row is created by the test under test (so we exercise Create).
func seedAccount(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	var id uuid.UUID
	err := pool.QueryRow(ctx, `
		INSERT INTO accounts (billing_email, country)
		VALUES ($1, 'TH')
		RETURNING id
	`, "test-"+uuid.NewString()+"@example.com").Scan(&id)
	if err != nil {
		t.Fatalf("seed account: %v", err)
	}
	t.Cleanup(func() {
		// ON DELETE CASCADE will reach subscriptions + subscription_events.
		_, _ = pool.Exec(context.Background(), `DELETE FROM accounts WHERE id = $1`, id)
	})
	return id
}

func countEvents(t *testing.T, pool *pgxpool.Pool, subID uuid.UUID, eventType string) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(), `
		SELECT COUNT(*) FROM subscription_events
		WHERE subscription_id = $1 AND event_type = $2
	`, subID, eventType).Scan(&n); err != nil {
		t.Fatalf("count events %s: %v", eventType, err)
	}
	return n
}

func TestRepository_LifecycleHappyPath(t *testing.T) {
	pool := setupTestDB(t)
	repo := NewRepository(pool)
	svc := NewService(repo)
	ctx := context.Background()
	accountID := seedAccount(t, pool)

	// 1. Create → pending_kyc
	sub, err := svc.CreateForAccount(ctx, accountID)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if sub.Status != StatusPendingKYC {
		t.Fatalf("initial status: want pending_kyc, got %s", sub.Status)
	}
	if got := countEvents(t, pool, sub.ID, "created"); got != 1 {
		t.Fatalf("created events: want 1, got %d", got)
	}

	// 2. ActivateTrial → trialing (also verifies trial_ends_at = +365d).
	before := time.Now()
	sub, err = svc.ActivateTrial(ctx, accountID, "first_booking")
	if err != nil {
		t.Fatalf("activate trial: %v", err)
	}
	if sub.Status != StatusTrialing {
		t.Fatalf("after activate: want trialing, got %s", sub.Status)
	}
	if sub.TrialStartedAt == nil || sub.TrialEndsAt == nil {
		t.Fatalf("trial timestamps unset: started=%v ends=%v", sub.TrialStartedAt, sub.TrialEndsAt)
	}
	gotDur := sub.TrialEndsAt.Sub(*sub.TrialStartedAt)
	if gotDur != trialDuration {
		t.Fatalf("trial duration: want %s, got %s", trialDuration, gotDur)
	}
	if sub.TrialStartedAt.Before(before.Add(-1 * time.Minute)) {
		t.Fatalf("trial started too early: %s (before=%s)", sub.TrialStartedAt, before)
	}
	if got := countEvents(t, pool, sub.ID, "trial_activated"); got != 1 {
		t.Fatalf("trial_activated events: want 1, got %d", got)
	}

	// 3. ActivateTrial again — idempotent.
	sub2, err := svc.ActivateTrial(ctx, accountID, "first_booking")
	if err != nil {
		t.Fatalf("activate trial idempotent: %v", err)
	}
	if sub2.Status != StatusTrialing {
		t.Fatalf("after re-activate: want trialing, got %s", sub2.Status)
	}
	if got := countEvents(t, pool, sub.ID, "trial_activated"); got != 1 {
		t.Fatalf("trial_activated events after re-call: want 1, got %d", got)
	}

	// 4. MarkTrialEnding → trial_ending
	sub, err = svc.MarkTrialEnding(ctx, accountID)
	if err != nil {
		t.Fatalf("mark trial ending: %v", err)
	}
	if sub.Status != StatusTrialEnding {
		t.Fatalf("status: want trial_ending, got %s", sub.Status)
	}
	if got := countEvents(t, pool, sub.ID, "trial_ending"); got != 1 {
		t.Fatalf("trial_ending events: want 1, got %d", got)
	}

	// 5. MarkTrialLapsed → trial_lapsed
	sub, err = svc.MarkTrialLapsed(ctx, accountID)
	if err != nil {
		t.Fatalf("mark trial lapsed: %v", err)
	}
	if sub.Status != StatusTrialLapsed {
		t.Fatalf("status: want trial_lapsed, got %s", sub.Status)
	}
	if got := countEvents(t, pool, sub.ID, "trial_lapsed"); got != 1 {
		t.Fatalf("trial_lapsed events: want 1, got %d", got)
	}

	// 6. Suspend → suspended (and suspended_at set).
	sub, err = svc.Suspend(ctx, accountID)
	if err != nil {
		t.Fatalf("suspend: %v", err)
	}
	if sub.Status != StatusSuspended {
		t.Fatalf("status: want suspended, got %s", sub.Status)
	}
	if sub.SuspendedAt == nil {
		t.Fatalf("suspended_at must be set after suspend")
	}
	if got := countEvents(t, pool, sub.ID, "suspended"); got != 1 {
		t.Fatalf("suspended events: want 1, got %d", got)
	}

	// Total events should equal the transitions we made + the initial 'created'.
	var total int
	if err := pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM subscription_events WHERE subscription_id = $1
	`, sub.ID).Scan(&total); err != nil {
		t.Fatalf("total events: %v", err)
	}
	const want = 5 // created, trial_activated, trial_ending, trial_lapsed, suspended
	if total != want {
		t.Fatalf("total events: want %d, got %d", want, total)
	}
}

func TestRepository_EnsureIsIdempotent(t *testing.T) {
	pool := setupTestDB(t)
	repo := NewRepository(pool)
	ctx := context.Background()
	accountID := seedAccount(t, pool)

	a, err := repo.Ensure(ctx, accountID)
	if err != nil {
		t.Fatalf("ensure 1: %v", err)
	}
	b, err := repo.Ensure(ctx, accountID)
	if err != nil {
		t.Fatalf("ensure 2: %v", err)
	}
	if a.ID != b.ID {
		t.Fatalf("ensure returned different rows: %s vs %s", a.ID, b.ID)
	}
	// Exactly one 'created' event despite two Ensure calls.
	if got := countEvents(t, pool, a.ID, "created"); got != 1 {
		t.Fatalf("created events: want 1, got %d", got)
	}
}

func TestRepository_CreateDuplicateReturnsAlreadyExists(t *testing.T) {
	pool := setupTestDB(t)
	repo := NewRepository(pool)
	ctx := context.Background()
	accountID := seedAccount(t, pool)

	if _, err := repo.Create(ctx, accountID); err != nil {
		t.Fatalf("first create: %v", err)
	}
	_, err := repo.Create(ctx, accountID)
	if !errors.Is(err, ErrAlreadyExists) {
		t.Fatalf("want ErrAlreadyExists, got %v", err)
	}
}

func TestRepository_TransitionInvalidStateRejected(t *testing.T) {
	pool := setupTestDB(t)
	repo := NewRepository(pool)
	ctx := context.Background()
	accountID := seedAccount(t, pool)

	if _, err := repo.Create(ctx, accountID); err != nil {
		t.Fatalf("create: %v", err)
	}
	// pending_kyc → active is illegal per state machine.
	_, err := repo.Transition(ctx, accountID, StatusPendingKYC, StatusActive,
		"bogus", "system", nil, TransitionUpdate{}, nil)
	if !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("want ErrInvalidTransition, got %v", err)
	}
}

func TestRepository_UpdatePaymentMethod(t *testing.T) {
	pool := setupTestDB(t)
	repo := NewRepository(pool)
	svc := NewService(repo)
	ctx := context.Background()
	accountID := seedAccount(t, pool)

	if _, err := svc.CreateForAccount(ctx, accountID); err != nil {
		t.Fatalf("create: %v", err)
	}
	sub, err := svc.UpdatePaymentMethod(ctx, accountID, nil, UpdatePaymentMethodRequest{
		Provider: ProviderStripe,
		MethodID: "pm_test_123",
		Last4:    "4242",
		Brand:    "visa",
	})
	if err != nil {
		t.Fatalf("update pm: %v", err)
	}
	if sub.PaymentMethodID != "pm_test_123" {
		t.Fatalf("method id: got %q", sub.PaymentMethodID)
	}
	if sub.PaymentMethodLast4 != "4242" || sub.PaymentMethodBrand != "visa" {
		t.Fatalf("display attrs: last4=%q brand=%q", sub.PaymentMethodLast4, sub.PaymentMethodBrand)
	}
	if got := countEvents(t, pool, sub.ID, "payment_method_updated"); got != 1 {
		t.Fatalf("payment_method_updated events: want 1, got %d", got)
	}
}

func TestRepository_GetNotFound(t *testing.T) {
	pool := setupTestDB(t)
	repo := NewRepository(pool)
	ctx := context.Background()

	_, err := repo.GetByAccount(ctx, uuid.New())
	if !errors.Is(err, ErrSubscriptionNotFound) {
		t.Fatalf("want ErrSubscriptionNotFound, got %v", err)
	}
}
