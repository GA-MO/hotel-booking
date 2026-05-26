package auth

import (
	"time"

	"github.com/google/uuid"
)

// User represents an authenticated staff user (owner / manager / front_desk / read_only).
type User struct {
	ID              uuid.UUID  `json:"id"`
	AccountID       uuid.UUID  `json:"account_id"`
	Email           string     `json:"email"`
	Name            string     `json:"name"`
	Role            string     `json:"role"`
	Locale          string     `json:"locale"`
	EmailVerifiedAt *time.Time `json:"email_verified_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
}

// Identity is the subject extracted from a verified access token; injected into request context.
type Identity struct {
	UserID    uuid.UUID
	AccountID uuid.UUID
	Role      string
}

// Session is a refresh-token record.
type Session struct {
	ID         uuid.UUID
	UserID     uuid.UUID
	ExpiresAt  time.Time
	RevokedAt  *time.Time
	RotatedTo  *uuid.UUID
	CreatedAt  time.Time
	LastUsedAt time.Time
}

// ----- request DTOs -----

type SignupRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
	Locale   string `json:"locale,omitempty"`
	Country  string `json:"country,omitempty"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type LogoutRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// ----- response DTOs -----

type AuthResponse struct {
	User                 User   `json:"user"`
	AccessToken          string `json:"access_token"`
	RefreshToken         string `json:"refresh_token"`
	AccessTokenExpiresIn int    `json:"access_token_expires_in"`
}

type MeResponse struct {
	User User `json:"user"`
}
