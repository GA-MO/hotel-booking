package roomtype

import "errors"

var (
	ErrRoomTypeNotFound      = errors.New("room type not found")
	ErrPhotoNotFound         = errors.New("photo not found")
	ErrInvalidName           = errors.New("invalid name")
	ErrInvalidCapacity       = errors.New("invalid max_occupancy")
	ErrInvalidInventory      = errors.New("invalid total_inventory")
	ErrInvalidBaseRate       = errors.New("invalid base_rate")
	ErrInvalidCurrency       = errors.New("invalid base_currency")
	ErrInvalidDisplayOrder   = errors.New("invalid display_order")
	ErrInvalidStorageKey     = errors.New("invalid storage_key")
	ErrCoverConflict         = errors.New("another cover photo already exists")
	ErrForbidden             = errors.New("forbidden")
)
