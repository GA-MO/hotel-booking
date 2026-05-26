package hotel

import (
	"time"

	"github.com/google/uuid"
)

// Hotel is a single property owned by an account.
type Hotel struct {
	ID            uuid.UUID `json:"id"`
	AccountID     uuid.UUID `json:"account_id"`
	Slug          string    `json:"slug"`
	Name          string    `json:"name"`
	HotelType     string    `json:"hotel_type,omitempty"`
	Description   string    `json:"description,omitempty"`

	AddressLine string   `json:"address_line,omitempty"`
	City        string   `json:"city,omitempty"`
	Country     string   `json:"country,omitempty"`
	PostalCode  string   `json:"postal_code,omitempty"`
	Latitude    *float64 `json:"latitude,omitempty"`
	Longitude   *float64 `json:"longitude,omitempty"`

	Phone  string `json:"phone,omitempty"`
	Email  string `json:"email,omitempty"`
	LineID string `json:"line_id,omitempty"`

	Timezone     string `json:"timezone"`
	BaseCurrency string `json:"base_currency"`

	CheckInTime  string `json:"check_in_time"`  // HH:MM:SS
	CheckOutTime string `json:"check_out_time"` // HH:MM:SS

	KYCStatus string `json:"kyc_status"` // pending | submitted | approved | rejected
	Status    string `json:"status"`     // test | live | suspended | archived

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// CreateRequest is the body for POST /v1/hotels.
type CreateRequest struct {
	Slug         string `json:"slug"`
	Name         string `json:"name"`
	HotelType    string `json:"hotel_type,omitempty"`
	Country      string `json:"country,omitempty"`
	Timezone     string `json:"timezone,omitempty"`
	BaseCurrency string `json:"base_currency,omitempty"`
}

// UpdateRequest is the body for PATCH /v1/hotels/{id}. All fields optional;
// only non-nil fields are written. Slug is intentionally not updatable here —
// changing the slug breaks ad-spend URLs, so it requires a dedicated endpoint.
type UpdateRequest struct {
	Name         *string  `json:"name,omitempty"`
	HotelType    *string  `json:"hotel_type,omitempty"`
	Description  *string  `json:"description,omitempty"`
	AddressLine  *string  `json:"address_line,omitempty"`
	City         *string  `json:"city,omitempty"`
	Country      *string  `json:"country,omitempty"`
	PostalCode   *string  `json:"postal_code,omitempty"`
	Latitude     *float64 `json:"latitude,omitempty"`
	Longitude    *float64 `json:"longitude,omitempty"`
	Phone        *string  `json:"phone,omitempty"`
	Email        *string  `json:"email,omitempty"`
	LineID       *string  `json:"line_id,omitempty"`
	Timezone     *string  `json:"timezone,omitempty"`
	BaseCurrency *string  `json:"base_currency,omitempty"`
	CheckInTime  *string  `json:"check_in_time,omitempty"`
	CheckOutTime *string  `json:"check_out_time,omitempty"`
}

type SlugAvailableResponse struct {
	Slug      string `json:"slug"`
	Available bool   `json:"available"`
}

type ListResponse struct {
	Hotels []Hotel `json:"hotels"`
}
