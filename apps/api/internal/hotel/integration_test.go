package hotel_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/GA-MO/hotel-booking/apps/api/internal/hotel"
	"github.com/GA-MO/hotel-booking/apps/api/internal/testdb"
)

func TestRepo_CreateAndSlugCollision(t *testing.T) {
	pool := testdb.Open(t)
	testdb.Truncate(t, pool)
	f := testdb.SeedBase(t, pool, 1) // creates one account + one hotel with slug "smoke-hotel"
	repo := hotel.NewRepository(pool)
	ctx := context.Background()

	// Different account creates a hotel with a unique slug — should succeed.
	otherAccount := mkAccount(t, pool)
	h, err := repo.Create(ctx, otherAccount, "another-hotel", "Another", "boutique", "TH", "Asia/Bangkok", "THB")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if h.Slug != "another-hotel" {
		t.Errorf("slug: want %q, got %q", "another-hotel", h.Slug)
	}

	// Same account or different — claiming the seeded slug must fail.
	if _, err := repo.Create(ctx, otherAccount, f.HotelSlug, "Dup", "", "", "Asia/Bangkok", "THB"); !errors.Is(err, hotel.ErrSlugAlreadyTaken) {
		t.Errorf("expected ErrSlugAlreadyTaken, got %v", err)
	}
}

func TestRepo_GetByID_TenantIsolation(t *testing.T) {
	pool := testdb.Open(t)
	testdb.Truncate(t, pool)
	f := testdb.SeedBase(t, pool, 1)
	repo := hotel.NewRepository(pool)
	ctx := context.Background()

	// Owner can read own hotel.
	h, err := repo.GetByID(ctx, f.AccountID, f.HotelID)
	if err != nil {
		t.Fatalf("GetByID own: %v", err)
	}
	if h.ID != f.HotelID {
		t.Errorf("id mismatch")
	}

	// Different account → 404.
	stranger := mkAccount(t, pool)
	if _, err := repo.GetByID(ctx, stranger, f.HotelID); !errors.Is(err, hotel.ErrHotelNotFound) {
		t.Errorf("expected ErrHotelNotFound for stranger, got %v", err)
	}
}

func TestRepo_ListByAccount(t *testing.T) {
	pool := testdb.Open(t)
	testdb.Truncate(t, pool)
	f := testdb.SeedBase(t, pool, 1)
	repo := hotel.NewRepository(pool)
	ctx := context.Background()

	// Add a second hotel to the same account.
	if _, err := repo.Create(ctx, f.AccountID, "second", "Second", "", "", "Asia/Bangkok", "THB"); err != nil {
		t.Fatalf("Create second: %v", err)
	}
	list, err := repo.ListByAccount(ctx, f.AccountID)
	if err != nil {
		t.Fatalf("ListByAccount: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("expected 2 hotels for account, got %d", len(list))
	}

	// Empty account.
	stranger := mkAccount(t, pool)
	emptyList, err := repo.ListByAccount(ctx, stranger)
	if err != nil {
		t.Fatalf("ListByAccount stranger: %v", err)
	}
	if len(emptyList) != 0 {
		t.Errorf("stranger should have 0 hotels, got %d", len(emptyList))
	}
}

func TestRepo_SlugAvailable(t *testing.T) {
	pool := testdb.Open(t)
	testdb.Truncate(t, pool)
	f := testdb.SeedBase(t, pool, 1)
	repo := hotel.NewRepository(pool)
	ctx := context.Background()

	available, err := repo.SlugAvailable(ctx, "totally-fresh")
	if err != nil {
		t.Fatalf("SlugAvailable fresh: %v", err)
	}
	if !available {
		t.Errorf("fresh slug should be available")
	}

	taken, err := repo.SlugAvailable(ctx, f.HotelSlug)
	if err != nil {
		t.Fatalf("SlugAvailable taken: %v", err)
	}
	if taken {
		t.Errorf("seeded slug should be unavailable")
	}
}

func TestRepo_Update_TenantIsolation(t *testing.T) {
	pool := testdb.Open(t)
	testdb.Truncate(t, pool)
	f := testdb.SeedBase(t, pool, 1)
	repo := hotel.NewRepository(pool)
	ctx := context.Background()

	stranger := mkAccount(t, pool)
	newName := "Should Not Land"
	if _, err := repo.Update(ctx, stranger, f.HotelID, hotel.UpdateRequest{Name: &newName}); !errors.Is(err, hotel.ErrHotelNotFound) {
		t.Errorf("expected ErrHotelNotFound on cross-tenant update, got %v", err)
	}
}

// mkAccount creates a bare account and returns its id — used to exercise
// tenant-isolation in tests without going through the auth layer.
func mkAccount(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	err := pool.QueryRow(context.Background(),
		`INSERT INTO accounts (billing_email) VALUES ('x'||gen_random_uuid()::text||'@x.com') RETURNING id`).Scan(&id)
	if err != nil {
		t.Fatalf("mkAccount: %v", err)
	}
	return id
}
