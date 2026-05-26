package auth_test

import (
	"context"
	"errors"
	"net/netip"
	"testing"
	"time"

	"github.com/GA-MO/hotel-booking/apps/api/internal/auth"
	"github.com/GA-MO/hotel-booking/apps/api/internal/testdb"
)

func TestRepo_CreateAccountWithOwner(t *testing.T) {
	pool := testdb.Open(t)
	testdb.Truncate(t, pool)
	repo := auth.NewRepository(pool)
	ctx := context.Background()

	hash, err := auth.HashPassword("secret123")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}

	u, err := repo.CreateAccountWithOwner(ctx, "new@example.com", hash, "New Owner", "th", "TH")
	if err != nil {
		t.Fatalf("CreateAccountWithOwner: %v", err)
	}
	if u.Role != "owner" {
		t.Errorf("role: want owner, got %s", u.Role)
	}
	if u.Email != "new@example.com" {
		t.Errorf("email: want new@example.com, got %s", u.Email)
	}

	// Duplicate email must fail with ErrEmailAlreadyExists.
	if _, err := repo.CreateAccountWithOwner(ctx, "new@example.com", hash, "Dup", "", ""); !errors.Is(err, auth.ErrEmailAlreadyExists) {
		t.Errorf("dup email: want ErrEmailAlreadyExists, got %v", err)
	}
}

func TestRepo_GetUserByEmail_PasswordRoundTrip(t *testing.T) {
	pool := testdb.Open(t)
	testdb.Truncate(t, pool)
	repo := auth.NewRepository(pool)
	ctx := context.Background()

	hash, _ := auth.HashPassword("correct horse battery staple")
	if _, err := repo.CreateAccountWithOwner(ctx, "user@example.com", hash, "User", "", ""); err != nil {
		t.Fatalf("seed: %v", err)
	}

	_, gotHash, err := repo.GetUserByEmail(ctx, "user@example.com")
	if err != nil {
		t.Fatalf("GetUserByEmail: %v", err)
	}
	ok, err := auth.VerifyPassword(gotHash, "correct horse battery staple")
	if err != nil || !ok {
		t.Errorf("VerifyPassword on stored hash: ok=%v err=%v", ok, err)
	}

	if _, _, err := repo.GetUserByEmail(ctx, "nobody@example.com"); !errors.Is(err, auth.ErrUserNotFound) {
		t.Errorf("missing user: want ErrUserNotFound, got %v", err)
	}
}

func TestRepo_SessionRotation(t *testing.T) {
	pool := testdb.Open(t)
	testdb.Truncate(t, pool)
	repo := auth.NewRepository(pool)
	ctx := context.Background()

	hash, _ := auth.HashPassword("secret123")
	u, err := repo.CreateAccountWithOwner(ctx, "rot@example.com", hash, "Rot", "", "")
	if err != nil {
		t.Fatalf("seed: %v", err)
	}

	ip, _ := netip.ParseAddr("127.0.0.1")
	s1, err := repo.CreateSession(ctx, u.ID, "hash1", "ua/1", ip, time.Now().Add(24*time.Hour))
	if err != nil {
		t.Fatalf("CreateSession s1: %v", err)
	}
	s2, err := repo.CreateSession(ctx, u.ID, "hash2", "ua/2", ip, time.Now().Add(24*time.Hour))
	if err != nil {
		t.Fatalf("CreateSession s2: %v", err)
	}

	// Rotate s1 to s2.
	if err := repo.RevokeSession(ctx, s1.ID, &s2.ID); err != nil {
		t.Fatalf("RevokeSession: %v", err)
	}

	got, err := repo.GetSessionByTokenHash(ctx, "hash1")
	if err != nil {
		t.Fatalf("GetSessionByTokenHash: %v", err)
	}
	if got.RevokedAt == nil {
		t.Errorf("revoked_at not set")
	}
	if got.RotatedTo == nil || *got.RotatedTo != s2.ID {
		t.Errorf("rotated_to: want %s, got %v", s2.ID, got.RotatedTo)
	}

	// RevokeAllUserSessions should revoke s2 too.
	if err := repo.RevokeAllUserSessions(ctx, u.ID); err != nil {
		t.Fatalf("RevokeAllUserSessions: %v", err)
	}
	got2, err := repo.GetSessionByTokenHash(ctx, "hash2")
	if err != nil {
		t.Fatalf("GetSessionByTokenHash s2: %v", err)
	}
	if got2.RevokedAt == nil {
		t.Errorf("s2 should be revoked after RevokeAllUserSessions")
	}
}
