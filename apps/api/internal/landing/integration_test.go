package landing_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/GA-MO/hotel-booking/apps/api/internal/landing"
	"github.com/GA-MO/hotel-booking/apps/api/internal/testdb"
)

// TestRepo_GetPublishedBySlug exercises the public landing read path end-to-end
// against real Postgres. The happy path asserts that PublicHotelContext is fully
// populated — this is the test that would have caught the `currency` vs
// `base_currency` typo when GetPublishedBySlug was wired up.
//
// Negative cases verify the 404 envelope is preserved when:
//   - the hotel is not 'live' (suspended)
//   - the landing page is in 'draft' status
//   - the requested locale doesn't exist for this hotel
//
// Test name is prefixed with `TestRepo_` so `make test-integration` (which
// filters with `-run TestRepo`) picks it up. The test exercises Service +
// Repository together because the bug we want to catch (column typo) lives
// in the SQL inside the repository.
func TestRepo_GetPublishedBySlug(t *testing.T) {
	pool := testdb.Open(t)
	testdb.Truncate(t, pool)
	f := testdb.SeedBase(t, pool, 1) // status='live', timezone Asia/Bangkok, base_currency THB
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Set a PromptPay ID so we can assert the full hotel context renders.
	if _, err := pool.Exec(ctx, `
		UPDATE hotels SET promptpay_id = $1 WHERE id = $2
	`, "0812345678", f.HotelID); err != nil {
		t.Fatalf("set promptpay_id: %v", err)
	}

	repo := landing.NewRepository(pool)
	svc := landing.NewService(repo)

	const locale = "th"
	if _, err := repo.Upsert(ctx, f.HotelID, locale,
		landing.Branding{PrimaryColor: "#112233"},
		[]landing.Section{},
		landing.SEO{Title: "Welcome"},
		landing.Tracking{}); err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	if _, err := repo.Publish(ctx, f.HotelID, locale); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	// --- happy path ---
	t.Run("happy path returns full hotel context", func(t *testing.T) {
		resp, err := svc.GetPublishedBySlug(ctx, f.HotelSlug, locale)
		if err != nil {
			t.Fatalf("GetPublishedBySlug: %v", err)
		}
		if resp.Hotel.Timezone != "Asia/Bangkok" {
			t.Errorf("Hotel.Timezone: want Asia/Bangkok, got %q", resp.Hotel.Timezone)
		}
		// THB is CHAR(3) in Postgres so the value is the un-padded literal "THB".
		if resp.Hotel.Currency != "THB" {
			t.Errorf("Hotel.Currency: want THB, got %q (would catch the `currency` vs `base_currency` typo)", resp.Hotel.Currency)
		}
		if resp.Hotel.PromptPayID == nil || *resp.Hotel.PromptPayID != "0812345678" {
			got := "<nil>"
			if resp.Hotel.PromptPayID != nil {
				got = *resp.Hotel.PromptPayID
			}
			t.Errorf("Hotel.PromptPayID: want 0812345678, got %s", got)
		}
		if resp.Hotel.Slug != f.HotelSlug {
			t.Errorf("Hotel.Slug: want %q, got %q", f.HotelSlug, resp.Hotel.Slug)
		}
		if resp.Status != "published" {
			t.Errorf("Status: want published, got %q", resp.Status)
		}
	})

	// --- negative: hotel suspended → 404 ---
	t.Run("suspended hotel hides published landing", func(t *testing.T) {
		mustExec(t, pool, `UPDATE hotels SET status = 'suspended' WHERE id = $1`, f.HotelID)
		t.Cleanup(func() {
			mustExec(t, pool, `UPDATE hotels SET status = 'live' WHERE id = $1`, f.HotelID)
		})
		_, err := svc.GetPublishedBySlug(ctx, f.HotelSlug, locale)
		if !errors.Is(err, landing.ErrLandingPageNotFound) {
			t.Errorf("want ErrLandingPageNotFound, got %v", err)
		}
	})

	// --- negative: landing page in draft → 404 ---
	t.Run("draft landing page is undiscoverable", func(t *testing.T) {
		if _, err := repo.Unpublish(ctx, f.HotelID, locale); err != nil {
			t.Fatalf("Unpublish: %v", err)
		}
		t.Cleanup(func() {
			if _, err := repo.Publish(ctx, f.HotelID, locale); err != nil {
				t.Fatalf("re-Publish: %v", err)
			}
		})
		_, err := svc.GetPublishedBySlug(ctx, f.HotelSlug, locale)
		if !errors.Is(err, landing.ErrLandingPageNotFound) {
			t.Errorf("want ErrLandingPageNotFound, got %v", err)
		}
	})

	// --- negative: locale that doesn't exist → 404 ---
	t.Run("locale mismatch returns 404", func(t *testing.T) {
		_, err := svc.GetPublishedBySlug(ctx, f.HotelSlug, "en")
		if !errors.Is(err, landing.ErrLandingPageNotFound) {
			t.Errorf("want ErrLandingPageNotFound, got %v", err)
		}
	})
}

func mustExec(t *testing.T, pool *pgxpool.Pool, sql string, args ...any) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), sql, args...); err != nil {
		t.Fatalf("exec: %v", err)
	}
}
