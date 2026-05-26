package hotel

import "errors"

var (
	ErrHotelNotFound      = errors.New("hotel not found")
	ErrSlugAlreadyTaken   = errors.New("slug already taken")
	ErrInvalidSlug        = errors.New("invalid slug")
	ErrInvalidName        = errors.New("invalid name")
	ErrInvalidPromptPayID = errors.New("invalid promptpay id")
	ErrForbidden          = errors.New("forbidden")
	ErrInvalidRequest     = errors.New("invalid request")
	// ErrInvalidStateTransition is returned when GoLive/Suspend is called
	// from a status that doesn't allow it (e.g. archived → live).
	ErrInvalidStateTransition = errors.New("invalid status transition")
)
