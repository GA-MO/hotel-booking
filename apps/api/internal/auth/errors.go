package auth

import "errors"

var (
	ErrInvalidCredentials  = errors.New("invalid credentials")
	ErrEmailAlreadyExists  = errors.New("email already exists")
	ErrUserNotFound        = errors.New("user not found")
	ErrSessionNotFound     = errors.New("session not found")
	ErrSessionExpired      = errors.New("session expired")
	ErrSessionRevoked      = errors.New("session revoked")
	// ErrSessionReused is raised when a refresh token whose session was already
	// rotated is presented again — strong signal of token theft. Service layer
	// translates this into "revoke all sessions for the user" before returning
	// ErrSessionRevoked to the caller.
	ErrSessionReused       = errors.New("session reused")
	ErrInvalidToken        = errors.New("invalid token")
	ErrPasswordTooWeak     = errors.New("password too weak")
	ErrInvalidEmail        = errors.New("invalid email")
)
