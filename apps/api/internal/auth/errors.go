package auth

import "errors"

var (
	ErrInvalidCredentials  = errors.New("invalid credentials")
	ErrEmailAlreadyExists  = errors.New("email already exists")
	ErrUserNotFound        = errors.New("user not found")
	ErrSessionNotFound     = errors.New("session not found")
	ErrSessionExpired      = errors.New("session expired")
	ErrSessionRevoked      = errors.New("session revoked")
	ErrInvalidToken        = errors.New("invalid token")
	ErrPasswordTooWeak     = errors.New("password too weak")
	ErrInvalidEmail        = errors.New("invalid email")
)
