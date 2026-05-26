package upload

import (
	"time"

	"github.com/google/uuid"
)

// Kind discriminates between hotel-level and room-type photo uploads. The
// object key embeds this so cleanup-by-prefix is straightforward.
type Kind string

const (
	KindHotelPhoto    Kind = "hotel_photo"
	KindRoomTypePhoto Kind = "room_type_photo"
)

// MaxUploadBytes caps a single PUT at 10 MiB. We enforce this in the signer
// (via Content-Length-Range conditions where supported) AND at the FE.
// Real enforcement happens at the storage backend on the actual PUT, but
// rejecting oversized requests early saves a signed-URL round-trip.
const MaxUploadBytes = 10 * 1024 * 1024

// AllowedContentTypes is the whitelist of image MIME types we permit.
// Order matters only for the canonical error message.
var AllowedContentTypes = []string{
	"image/jpeg",
	"image/png",
	"image/webp",
}

// PresignRequest is the body for POST /v1/uploads/presign. `HotelID` is
// optional for `hotel_photo` (the FE may not yet have a hotel for an
// onboarding wizard step), but recommended whenever known so the object
// key carries it.
type PresignRequest struct {
	Kind        Kind       `json:"kind"`
	HotelID     *uuid.UUID `json:"hotel_id,omitempty"`
	ContentType string     `json:"content_type"`
	SizeBytes   int64      `json:"size_bytes"`
}

// PresignResponse is what the FE PUTs to. `Headers` are the headers the
// browser MUST send on the PUT; they're part of the canonical request and
// any deviation invalidates the signature.
type PresignResponse struct {
	UploadURL string            `json:"upload_url"`
	ObjectKey string            `json:"object_key"`
	PublicURL string            `json:"public_url"`
	Headers   map[string]string `json:"headers"`
	ExpiresAt time.Time         `json:"expires_at"`
}

// ImgproxyRequest is the body for POST /v1/uploads/imgproxy-url. The FE
// could compute these client-side, but doing it on the server keeps the
// `IMGPROXY_KEY` material secret.
type ImgproxyRequest struct {
	SourceURL string `json:"source_url"`
	Width     int    `json:"width,omitempty"`
	Height    int    `json:"height,omitempty"`
	Format    string `json:"format,omitempty"` // e.g. "webp", "jpg"; empty = auto
	Resize    string `json:"resize,omitempty"` // "fit" (default), "fill", "auto"
}

// ImgproxyResponse is the signed delivery URL.
type ImgproxyResponse struct {
	URL string `json:"url"`
}
