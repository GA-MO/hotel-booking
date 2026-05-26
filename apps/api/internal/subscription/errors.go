package subscription

import "errors"

var (
	ErrSubscriptionNotFound = errors.New("subscription not found")
	ErrAlreadyExists        = errors.New("subscription already exists for account")
	ErrInvalidTransition    = errors.New("invalid state transition")
	ErrInvalidProvider      = errors.New("invalid payment provider")
	ErrInvalidPaymentMethod = errors.New("invalid payment method")
	ErrInvalidRequest       = errors.New("invalid request")
	ErrForbidden            = errors.New("forbidden")
)
