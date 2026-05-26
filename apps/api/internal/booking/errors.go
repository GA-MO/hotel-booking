package booking

import "errors"

var (
	ErrBookingNotFound       = errors.New("booking not found")
	ErrInvalidDates          = errors.New("invalid dates")
	ErrInvalidRoomCount      = errors.New("invalid room count")
	ErrInvalidGuest          = errors.New("invalid guest info")
	ErrNoAvailability        = errors.New("no availability for the requested dates")
	ErrInvalidStateTransition = errors.New("invalid state transition")
	ErrAlreadyExpired        = errors.New("booking already expired")
	ErrAlreadyCancelled      = errors.New("booking already cancelled")
	ErrHotelNotLive          = errors.New("hotel not accepting bookings")
	ErrRoomTypeNotFound      = errors.New("room type not found")
)
