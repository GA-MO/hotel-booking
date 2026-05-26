package notification

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// openTestDB returns a pgxpool against TEST_DATABASE_URL or skips the test.
// Test rows must clean up after themselves via t.Cleanup — the schema is
// shared with the dev database.
func openTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set TEST_DATABASE_URL to run integration tests")
	}
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pool.Close() })
	return pool
}

// cleanupNotifications deletes the supplied notification rows.
func cleanupNotifications(t *testing.T, pool *pgxpool.Pool, ids ...uuid.UUID) {
	t.Helper()
	if len(ids) == 0 {
		return
	}
	ctx := context.Background()
	for _, id := range ids {
		if _, err := pool.Exec(ctx, `DELETE FROM notifications WHERE id = $1`, id); err != nil {
			t.Logf("cleanup %s: %v", id, err)
		}
	}
}

func TestRepository_EnqueueAndClaim(t *testing.T) {
	pool := openTestDB(t)
	repo := NewRepository(pool)
	ctx := context.Background()

	n, err := repo.Enqueue(ctx, Notification{
		Channel:   ChannelEmail,
		Template:  TemplateBookingCreated,
		Recipient: "claim@example.com",
		Payload:   map[string]any{"locale": "th", "reference": "REF-CLAIM"},
	})
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	t.Cleanup(func() { cleanupNotifications(t, pool, n.ID) })

	if n.Status != StatusQueued {
		t.Fatalf("status: want queued, got %s", n.Status)
	}
	if n.Attempts != 0 {
		t.Fatalf("attempts: want 0, got %d", n.Attempts)
	}

	claimed, err := repo.ClaimBatch(ctx, 10, time.Now())
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	found := false
	for _, c := range claimed {
		if c.ID == n.ID {
			found = true
			if c.Status != StatusSending {
				t.Fatalf("claimed status: want sending, got %s", c.Status)
			}
			if c.Attempts != 1 {
				t.Fatalf("claimed attempts: want 1, got %d", c.Attempts)
			}
		}
	}
	if !found {
		t.Fatalf("claimed batch did not include the row we just enqueued")
	}

	if err := repo.MarkSent(ctx, n.ID); err != nil {
		t.Fatalf("mark sent: %v", err)
	}
	got, err := repo.GetByID(ctx, n.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Status != StatusSent {
		t.Fatalf("post-mark status: want sent, got %s", got.Status)
	}
}

func TestRepository_ClaimSkipsLocked(t *testing.T) {
	pool := openTestDB(t)
	repo := NewRepository(pool)
	ctx := context.Background()

	// One row in the queue — two concurrent claims must not both see it.
	n, err := repo.Enqueue(ctx, Notification{
		Channel:   ChannelEmail,
		Template:  TemplateBookingCreated,
		Recipient: "skiplocked@example.com",
		Payload:   map[string]any{"locale": "th"},
	})
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	t.Cleanup(func() { cleanupNotifications(t, pool, n.ID) })

	var wg sync.WaitGroup
	results := make([][]Notification, 2)
	for i := 0; i < 2; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			out, err := repo.ClaimBatch(ctx, 10, time.Now())
			if err != nil {
				t.Errorf("claim %d: %v", i, err)
				return
			}
			results[i] = out
		}()
	}
	wg.Wait()

	count := 0
	for _, r := range results {
		for _, c := range r {
			if c.ID == n.ID {
				count++
			}
		}
	}
	if count != 1 {
		t.Fatalf("expected exactly one worker to claim the row, got %d", count)
	}
}

func TestRepository_MarkFailedRetryThenDead(t *testing.T) {
	pool := openTestDB(t)
	repo := NewRepository(pool)
	ctx := context.Background()

	n, err := repo.Enqueue(ctx, Notification{
		Channel:   ChannelEmail,
		Template:  TemplateBookingCreated,
		Recipient: "retry@example.com",
		Payload:   map[string]any{"locale": "th"},
	})
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	t.Cleanup(func() { cleanupNotifications(t, pool, n.ID) })

	// Fail it MaxAttempts times — must transition queued → queued (retry) until
	// the final attempt where it lands in 'dead'.
	for i := 0; i < MaxAttempts; i++ {
		// Each loop simulates worker: claim → fail.
		claimed, err := repo.ClaimBatch(ctx, 10, time.Now().Add(time.Hour))
		if err != nil {
			t.Fatalf("claim iter %d: %v", i, err)
		}
		// Confirm our row is among the claimed (other tests may add rows in
		// parallel — we don't depend on length).
		found := false
		for _, c := range claimed {
			if c.ID == n.ID {
				found = true
			}
		}
		if !found {
			t.Fatalf("iter %d: row not in claim batch", i)
		}
		if err := repo.MarkFailed(ctx, n.ID, "boom", false); err != nil {
			t.Fatalf("mark failed iter %d: %v", i, err)
		}
	}

	got, err := repo.GetByID(ctx, n.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Status != StatusDead {
		t.Fatalf("status after %d failures: want dead, got %s", MaxAttempts, got.Status)
	}
	if got.Attempts < MaxAttempts {
		t.Fatalf("attempts: want >= %d, got %d", MaxAttempts, got.Attempts)
	}
	if got.LastError == "" {
		t.Fatalf("last_error should be populated")
	}
}

func TestRepository_ListLatest(t *testing.T) {
	pool := openTestDB(t)
	repo := NewRepository(pool)
	ctx := context.Background()

	ids := make([]uuid.UUID, 0, 3)
	for i := 0; i < 3; i++ {
		n, err := repo.Enqueue(ctx, Notification{
			Channel:   ChannelEmail,
			Template:  TemplateBookingCreated,
			Recipient: "list@example.com",
			Payload:   map[string]any{"locale": "th"},
		})
		if err != nil {
			t.Fatalf("enqueue %d: %v", i, err)
		}
		ids = append(ids, n.ID)
	}
	t.Cleanup(func() { cleanupNotifications(t, pool, ids...) })

	out, err := repo.ListLatest(ctx, 100)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	seen := map[uuid.UUID]bool{}
	for _, n := range out {
		seen[n.ID] = true
	}
	for _, id := range ids {
		if !seen[id] {
			t.Fatalf("id %s missing from ListLatest", id)
		}
	}
}
