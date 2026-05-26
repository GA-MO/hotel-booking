package pricing

import "errors"

var (
	// ErrHotelNotFound is returned when a hotel is not found, deleted, or owned
	// by another account (we treat cross-tenant access as not-found to avoid
	// leaking existence).
	ErrHotelNotFound = errors.New("hotel not found")

	// ErrRoomTypeNotFound covers missing/deleted/foreign-owned room types.
	ErrRoomTypeNotFound = errors.New("room type not found")

	// ErrRuleNotFound is returned when a pricing rule does not exist within the
	// caller's account scope.
	ErrRuleNotFound = errors.New("pricing rule not found")

	// ErrInvalidDate is returned for malformed or empty date strings.
	ErrInvalidDate = errors.New("invalid date")

	// ErrInvalidDateRange is returned when check_out is not strictly after check_in.
	ErrInvalidDateRange = errors.New("invalid date range")

	// ErrInvalidRooms is returned when rooms count is < 1.
	ErrInvalidRooms = errors.New("invalid rooms count")

	// ErrInvalidRequest is a generic validation failure (missing field,
	// malformed UUID, etc).
	ErrInvalidRequest = errors.New("invalid request")

	// ErrInvalidRule is returned when a rule's fields are inconsistent with its
	// declared rule_type (e.g. season without start/end dates).
	ErrInvalidRule = errors.New("invalid pricing rule")

	// ErrInvalidOverride is returned when an availability override has
	// inconsistent fields (e.g. min_nights > max_nights).
	ErrInvalidOverride = errors.New("invalid availability override")
)
