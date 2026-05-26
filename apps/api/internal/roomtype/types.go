package roomtype

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// RoomType is a sellable category of rooms within a hotel.
// Inventory is tracked at this level (not per physical room unit).
type RoomType struct {
	ID          uuid.UUID `json:"id"`
	HotelID     uuid.UUID `json:"hotel_id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`

	TotalInventory int             `json:"total_inventory"`
	MaxOccupancy   int             `json:"max_occupancy"`
	SizeSqm        *float64        `json:"size_sqm,omitempty"`
	BedConfig      json.RawMessage `json:"bed_config"`
	Amenities      json.RawMessage `json:"amenities"`

	BaseRate     float64 `json:"base_rate"`
	BaseCurrency string  `json:"base_currency"`

	DisplayOrder int  `json:"display_order"`
	Enabled      bool `json:"enabled"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// CreateRequest is the body for POST /v1/hotels/{hotel_id}/room-types.
type CreateRequest struct {
	Name           string          `json:"name"`
	Description    string          `json:"description,omitempty"`
	TotalInventory int             `json:"total_inventory"`
	MaxOccupancy   int             `json:"max_occupancy"`
	SizeSqm        *float64        `json:"size_sqm,omitempty"`
	BedConfig      json.RawMessage `json:"bed_config,omitempty"`
	Amenities      json.RawMessage `json:"amenities,omitempty"`
	BaseRate       float64         `json:"base_rate"`
	BaseCurrency   string          `json:"base_currency,omitempty"`
	DisplayOrder   int             `json:"display_order,omitempty"`
}

// UpdateRequest is a PATCH-style partial update. Only non-nil fields are written.
type UpdateRequest struct {
	Name           *string          `json:"name,omitempty"`
	Description    *string          `json:"description,omitempty"`
	TotalInventory *int             `json:"total_inventory,omitempty"`
	MaxOccupancy   *int             `json:"max_occupancy,omitempty"`
	SizeSqm        *float64         `json:"size_sqm,omitempty"`
	BedConfig      *json.RawMessage `json:"bed_config,omitempty"`
	Amenities      *json.RawMessage `json:"amenities,omitempty"`
	BaseRate       *float64         `json:"base_rate,omitempty"`
	BaseCurrency   *string          `json:"base_currency,omitempty"`
	DisplayOrder   *int             `json:"display_order,omitempty"`
	Enabled        *bool            `json:"enabled,omitempty"`
}

type ListResponse struct {
	RoomTypes []RoomType `json:"room_types"`
}

// Photo is a single image attached to either a hotel or a room type.
// HotelID and RoomTypeID are mutually exclusive — exactly one is set per row,
// determined by which table backs the record.
type Photo struct {
	ID           uuid.UUID  `json:"id"`
	HotelID      *uuid.UUID `json:"hotel_id,omitempty"`
	RoomTypeID   *uuid.UUID `json:"room_type_id,omitempty"`
	StorageKey   string     `json:"storage_key"`
	Caption      string     `json:"caption,omitempty"`
	AltText      string     `json:"alt_text,omitempty"`
	Width        *int       `json:"width,omitempty"`
	Height       *int       `json:"height,omitempty"`
	DisplayOrder int        `json:"display_order"`
	IsCover      bool       `json:"is_cover"`
	CreatedAt    time.Time  `json:"created_at"`
}

// CreatePhotoRequest attaches a previously-uploaded asset (identified by its
// storage key) to a hotel or room type.
type CreatePhotoRequest struct {
	StorageKey   string `json:"storage_key"`
	Caption      string `json:"caption,omitempty"`
	AltText      string `json:"alt_text,omitempty"`
	Width        *int   `json:"width,omitempty"`
	Height       *int   `json:"height,omitempty"`
	DisplayOrder int    `json:"display_order,omitempty"`
	IsCover      bool   `json:"is_cover,omitempty"`
}

// UpdatePhotoRequest is a PATCH-style update for an existing photo row.
type UpdatePhotoRequest struct {
	Caption      *string `json:"caption,omitempty"`
	AltText      *string `json:"alt_text,omitempty"`
	DisplayOrder *int    `json:"display_order,omitempty"`
	IsCover      *bool   `json:"is_cover,omitempty"`
}

type PhotoListResponse struct {
	Photos []Photo `json:"photos"`
}
