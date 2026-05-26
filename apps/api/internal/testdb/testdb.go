// Package testdb provides helpers for repository-level integration tests.
//
// Tests are skipped if TEST_DATABASE_URL is not set. The expected workflow:
//
//   docker compose up -d postgres
//   make -C apps/api migrate-up
//   TEST_DATABASE_URL='postgres://hotel:hotel@localhost:5432/hotel_booking?sslmode=disable' \
//     go test ./... -tags=integration -race
//
// Tests can call Truncate(t) inside their setup to start from an empty schema.
package testdb

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

const skipMsg = "TEST_DATABASE_URL not set — integration tests skipped"

// Open returns a pgx pool to the configured test database. Caller is responsible
// for cleaning up via Truncate or t.Cleanup.
func Open(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip(skipMsg)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatalf("pgxpool.New: %v", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Fatalf("ping: %v — is Postgres running and migrations applied?", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// Truncate wipes every domain table — caller should run this in test setup so
// each test sees a clean schema. Order matters: dependents first.
func Truncate(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	const sql = `
		TRUNCATE TABLE
		  booking_events, bookings,
		  pricing_rules, availability_overrides,
		  landing_pages,
		  room_type_photos, hotel_photos,
		  room_types, hotels,
		  sessions, users, accounts
		RESTART IDENTITY CASCADE
	`
	if _, err := pool.Exec(ctx, sql); err != nil {
		t.Fatalf("truncate: %v", err)
	}
}

// Fixtures is the result of SeedBase — a small set of related rows that most
// integration tests need (one account with one hotel and one room type).
type Fixtures struct {
	AccountID  uuid.UUID
	UserID     uuid.UUID
	HotelID    uuid.UUID
	HotelSlug  string
	RoomTypeID uuid.UUID
}

// SeedBase inserts:
//   - 1 account
//   - 1 user (owner role, no real password hash)
//   - 1 hotel (status='live' so it accepts public bookings)
//   - 1 room type with totalInventory rooms at 1500 THB/night
//
// Returns the IDs for use in tests.
func SeedBase(t *testing.T, pool *pgxpool.Pool, totalInventory int) Fixtures {
	t.Helper()
	ctx := context.Background()
	var f Fixtures

	if err := pool.QueryRow(ctx, `
		INSERT INTO accounts (billing_email, country)
		VALUES ('owner@example.com', 'TH')
		RETURNING id
	`).Scan(&f.AccountID); err != nil {
		t.Fatalf("insert account: %v", err)
	}

	if err := pool.QueryRow(ctx, `
		INSERT INTO users (account_id, email, password_hash, name, role)
		VALUES ($1, 'owner@example.com', 'x', 'Owner', 'owner')
		RETURNING id
	`, f.AccountID).Scan(&f.UserID); err != nil {
		t.Fatalf("insert user: %v", err)
	}

	f.HotelSlug = "smoke-hotel"
	if err := pool.QueryRow(ctx, `
		INSERT INTO hotels (account_id, slug, name, status, kyc_status, timezone, base_currency)
		VALUES ($1, $2, 'Smoke Hotel', 'live', 'approved', 'Asia/Bangkok', 'THB')
		RETURNING id
	`, f.AccountID, f.HotelSlug).Scan(&f.HotelID); err != nil {
		t.Fatalf("insert hotel: %v", err)
	}

	if err := pool.QueryRow(ctx, `
		INSERT INTO room_types (hotel_id, name, max_occupancy, total_inventory, base_rate, base_currency)
		VALUES ($1, 'Standard Double', 2, $2, 1500, 'THB')
		RETURNING id
	`, f.HotelID, totalInventory).Scan(&f.RoomTypeID); err != nil {
		t.Fatalf("insert room_type: %v", err)
	}

	return f
}
