package auth

import (
	"errors"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func newTestJWT(t *testing.T, accessTTL time.Duration) *JWT {
	t.Helper()
	return NewJWT("test-secret-do-not-use-in-prod", accessTTL, time.Hour)
}

func newTestIdentity(t *testing.T) Identity {
	t.Helper()
	return Identity{
		UserID:    uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		AccountID: uuid.MustParse("22222222-2222-2222-2222-222222222222"),
		Role:      "owner",
	}
}

func TestJWTRoundTrip(t *testing.T) {
	j := newTestJWT(t, 5*time.Minute)
	want := newTestIdentity(t)

	tok, err := j.SignAccess(want)
	if err != nil {
		t.Fatalf("SignAccess: %v", err)
	}
	if tok == "" {
		t.Fatal("SignAccess returned empty token")
	}

	got, err := j.ParseAccess(tok)
	if err != nil {
		t.Fatalf("ParseAccess: %v", err)
	}
	if got != want {
		t.Errorf("identity mismatch: got %+v want %+v", got, want)
	}
}

func TestParseAccessRejectsWrongSecret(t *testing.T) {
	signer := newTestJWT(t, 5*time.Minute)
	verifier := NewJWT("a-completely-different-secret", 5*time.Minute, time.Hour)

	tok, err := signer.SignAccess(newTestIdentity(t))
	if err != nil {
		t.Fatalf("SignAccess: %v", err)
	}

	if _, err := verifier.ParseAccess(tok); err == nil {
		t.Fatal("expected error parsing token signed with different secret, got nil")
	} else if !errors.Is(err, ErrInvalidToken) {
		t.Errorf("expected ErrInvalidToken, got %v", err)
	}
}

func TestParseAccessRejectsExpired(t *testing.T) {
	j := newTestJWT(t, -time.Minute) // already expired at sign time
	tok, err := j.SignAccess(newTestIdentity(t))
	if err != nil {
		t.Fatalf("SignAccess: %v", err)
	}
	if _, err := j.ParseAccess(tok); err == nil {
		t.Fatal("expected error for expired token, got nil")
	} else if !errors.Is(err, ErrInvalidToken) {
		t.Errorf("expected ErrInvalidToken, got %v", err)
	}
}

func TestParseAccessRejectsGarbage(t *testing.T) {
	j := newTestJWT(t, 5*time.Minute)
	cases := []string{
		"",
		"not-a-jwt",
		"aaaa.bbbb.cccc",
		"....",
	}
	for _, in := range cases {
		in := in
		t.Run(in, func(t *testing.T) {
			if _, err := j.ParseAccess(in); err == nil {
				t.Errorf("expected error for garbage input %q, got nil", in)
			}
		})
	}
}

func TestParseAccessRejectsBadSubject(t *testing.T) {
	// Craft a JWT with a non-UUID subject so ParseAccess fails at uuid.Parse(sub).
	j := newTestJWT(t, 5*time.Minute)
	claims := AccessClaims{
		AccountID: "22222222-2222-2222-2222-222222222222",
		Role:      "owner",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "not-a-uuid",
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(5 * time.Minute)),
		},
	}
	tok, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(j.secret)
	if err != nil {
		t.Fatalf("hand-crafted sign: %v", err)
	}

	if _, err := j.ParseAccess(tok); err == nil {
		t.Fatal("expected error for bad subject UUID, got nil")
	} else if !errors.Is(err, ErrInvalidToken) {
		t.Errorf("expected ErrInvalidToken, got %v", err)
	}
}

func TestParseAccessRejectsBadAccountID(t *testing.T) {
	j := newTestJWT(t, 5*time.Minute)
	claims := AccessClaims{
		AccountID: "not-a-uuid",
		Role:      "owner",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "11111111-1111-1111-1111-111111111111",
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(5 * time.Minute)),
		},
	}
	tok, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(j.secret)
	if err != nil {
		t.Fatalf("hand-crafted sign: %v", err)
	}

	if _, err := j.ParseAccess(tok); err == nil {
		t.Fatal("expected error for bad account UUID, got nil")
	} else if !errors.Is(err, ErrInvalidToken) {
		t.Errorf("expected ErrInvalidToken, got %v", err)
	}
}

func TestJWTTTLAccessors(t *testing.T) {
	j := NewJWT("x", 7*time.Minute, 13*time.Hour)
	if got := j.AccessTTL(); got != 7*time.Minute {
		t.Errorf("AccessTTL: got %v want %v", got, 7*time.Minute)
	}
	if got := j.RefreshTTL(); got != 13*time.Hour {
		t.Errorf("RefreshTTL: got %v want %v", got, 13*time.Hour)
	}
}
