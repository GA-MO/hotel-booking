package landing

import "errors"

var (
	ErrLandingPageNotFound = errors.New("landing page not found")
	ErrHotelNotFound       = errors.New("hotel not found")
	ErrInvalidLocale       = errors.New("invalid locale")
	ErrInvalidColor        = errors.New("invalid color")
	ErrInvalidSection      = errors.New("invalid section")
	ErrInvalidSectionType  = errors.New("invalid section type")
	ErrInvalidRequest      = errors.New("invalid request")
	ErrForbidden           = errors.New("forbidden")
)
