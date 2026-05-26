package landing

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// LandingPage is the content payload for one hotel × locale combination.
// Layout is fixed in code; this row only carries the content/branding the
// hotel is allowed to customize.
type LandingPage struct {
	ID       uuid.UUID `json:"id"`
	HotelID  uuid.UUID `json:"hotel_id"`
	Locale   string    `json:"locale"`
	Status   string    `json:"status"`  // draft | published
	Version  int       `json:"version"` // bumped each draft write

	Branding Branding  `json:"branding"`
	Sections []Section `json:"sections"`
	SEO      SEO       `json:"seo"`
	Tracking Tracking  `json:"tracking"`

	PublishedAt *time.Time `json:"published_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// Branding controls the visual identity of a landing page.
// Hotels customize logo + 2 brand colors + font family from a preset list;
// custom CSS is intentionally not supported.
type Branding struct {
	LogoURL      string `json:"logo_url,omitempty"`
	PrimaryColor string `json:"primary_color,omitempty"`
	AccentColor  string `json:"accent_color,omitempty"`
	FontFamily   string `json:"font_family,omitempty"`
}

// SEO holds per-locale meta tags rendered by the public landing page.
type SEO struct {
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
	OGImageURL  string `json:"og_image_url,omitempty"`
}

// Tracking holds 3rd-party pixel/tag IDs. The frontend fires events
// (PageView / ViewContent / InitiateCheckout / Purchase) using these.
type Tracking struct {
	FacebookPixelID       string `json:"facebook_pixel_id,omitempty"`
	GoogleAnalyticsID     string `json:"google_analytics_id,omitempty"`
	GoogleAdsConversionID string `json:"google_ads_conversion_id,omitempty"`
	GTMID                 string `json:"gtm_id,omitempty"`
	LineTagID             string `json:"line_tag_id,omitempty"`
	TikTokPixelID         string `json:"tiktok_pixel_id,omitempty"`
}

// Section is one block in the page. Content is intentionally opaque JSON so
// the section catalog can evolve without DB migrations.
type Section struct {
	Type    string          `json:"type"` // hero | gallery | about | rooms | ...
	Enabled bool            `json:"enabled"`
	Order   int             `json:"order"`
	Content json.RawMessage `json:"content"`
}

// UpdateRequest is the body for PUT /v1/hotels/{hotel_id}/landing/{locale}.
// Upsert semantics: full replacement of branding/sections/seo/tracking.
// All four fields are required (use empty objects / empty array to clear).
type UpdateRequest struct {
	Branding Branding  `json:"branding"`
	Sections []Section `json:"sections"`
	SEO      SEO       `json:"seo"`
	Tracking Tracking  `json:"tracking"`
}

// ListResponse is returned by GET /v1/hotels/{hotel_id}/landing — one entry
// per locale that exists for the hotel.
type ListResponse struct {
	LandingPages []LandingPage `json:"landing_pages"`
}
