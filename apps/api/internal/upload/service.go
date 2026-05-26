package upload

import (
	"context"
	"errors"
	"fmt"
	"mime"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Service generates presigned URLs and signs imgproxy delivery URLs. It is
// stateless and safe for concurrent use; all per-request data flows
// through the method arguments.
// HotelOwnershipCheck verifies the (accountID, hotelID) pair belongs together.
// Optional — when wired by the server, Presign rejects requests where the
// caller names a hotel they don't own, preventing object keys from being
// rooted at another tenant's hotel id (defence-in-depth; the account_id
// prefix already enforces top-level isolation).
type HotelOwnershipCheck func(ctx context.Context, accountID, hotelID uuid.UUID) (bool, error)

type Service struct {
	signer       *Signer
	imgproxy     *Imgproxy
	bucket       string
	publicBase   string // CDN-fronted prefix, e.g. https://cdn.example.com/hotel-photos
	expiresAfter time.Duration
	ownsHotel    HotelOwnershipCheck

	// now is injected for tests; production uses time.Now.
	now func() time.Time
}

// SetHotelOwnershipCheck wires the optional cross-module hook. Pass
// hotel.Service.OwnedBy on server boot. Called by Presign whenever the
// request names a hotel_id.
func (s *Service) SetHotelOwnershipCheck(fn HotelOwnershipCheck) { s.ownsHotel = fn }

// ServiceConfig wires the runtime dependencies. PublicBaseURL is what we
// return to the FE as the storage URL of the uploaded file — it's the
// CDN-fronted prefix (e.g. https://cdn.example.com/hotel-photos), not the
// signing endpoint. The two MAY be identical in dev where MinIO serves
// objects directly.
type ServiceConfig struct {
	Signer        *Signer
	Imgproxy      *Imgproxy
	Bucket        string
	PublicBaseURL string
	Expires       time.Duration // 0 = default 15m
}

func NewService(cfg ServiceConfig) *Service {
	exp := cfg.Expires
	if exp <= 0 {
		exp = 15 * time.Minute
	}
	return &Service{
		signer:       cfg.Signer,
		imgproxy:     cfg.Imgproxy,
		bucket:       cfg.Bucket,
		publicBase:   strings.TrimRight(cfg.PublicBaseURL, "/"),
		expiresAfter: exp,
		now:          time.Now,
	}
}

// Presign validates and produces a PUT URL the FE can use to upload directly
// to storage. Returns:
//   - ErrUnsupportedKind for unknown Kind or disallowed content type
//   - ErrSizeExceeded if size > MaxUploadBytes
//   - ErrNotConfigured if the signer is unconfigured (e.g. missing creds)
func (s *Service) Presign(ctx context.Context, accountID uuid.UUID, req PresignRequest) (*PresignResponse, error) {
	if s.signer == nil {
		return nil, ErrNotConfigured
	}
	if !isKindValid(req.Kind) {
		return nil, ErrUnsupportedKind
	}
	if !isContentTypeAllowed(req.ContentType) {
		return nil, ErrInvalidContentType
	}
	if req.SizeBytes <= 0 || req.SizeBytes > MaxUploadBytes {
		return nil, ErrSizeExceeded
	}
	if req.HotelID != nil && *req.HotelID != uuid.Nil && s.ownsHotel != nil {
		ok, err := s.ownsHotel(ctx, accountID, *req.HotelID)
		if err != nil {
			return nil, fmt.Errorf("verify hotel ownership: %w", err)
		}
		if !ok {
			return nil, ErrHotelNotOwned
		}
	}

	objectKey := deriveObjectKey(accountID, req.HotelID, req.Kind, req.ContentType, uuid.New())

	now := s.now()
	uploadURL, headers, err := s.signer.PresignPut(objectKey, req.ContentType, s.expiresAfter, now)
	if err != nil {
		return nil, fmt.Errorf("presign: %w", err)
	}

	publicURL := s.publicURLFor(objectKey)

	return &PresignResponse{
		UploadURL: uploadURL,
		ObjectKey: objectKey,
		PublicURL: publicURL,
		Headers:   headers,
		ExpiresAt: now.Add(s.expiresAfter).UTC(),
	}, nil
}

// ImgproxyURL returns a signed delivery URL for the given source. We expose
// this through the handler so the IMGPROXY_KEY material never leaves the
// server; FE just asks for a transform.
func (s *Service) ImgproxyURL(_ context.Context, req ImgproxyRequest) (*ImgproxyResponse, error) {
	if s.imgproxy == nil {
		return nil, ErrNotConfigured
	}
	if strings.TrimSpace(req.SourceURL) == "" {
		return nil, ErrInvalidRequest
	}
	t := Transform{
		Width:  req.Width,
		Height: req.Height,
		Resize: req.Resize,
		Format: req.Format,
	}
	return &ImgproxyResponse{URL: s.imgproxy.Sign(req.SourceURL, t)}, nil
}

// ----- helpers -----

func isKindValid(k Kind) bool {
	switch k {
	case KindHotelPhoto, KindRoomTypePhoto:
		return true
	}
	return false
}

func isContentTypeAllowed(ct string) bool {
	ct = strings.ToLower(strings.TrimSpace(ct))
	for _, a := range AllowedContentTypes {
		if ct == a {
			return true
		}
	}
	return false
}

// extensionFor returns a lowercase extension WITHOUT the leading dot for the
// given content type. Falls back to "bin" if mime registry doesn't know it
// (shouldn't happen — we whitelist content types).
func extensionFor(contentType string) string {
	switch strings.ToLower(strings.TrimSpace(contentType)) {
	case "image/jpeg":
		return "jpg"
	case "image/png":
		return "png"
	case "image/webp":
		return "webp"
	}
	// Last-resort: ask the stdlib.
	exts, _ := mime.ExtensionsByType(contentType)
	if len(exts) > 0 {
		return strings.TrimPrefix(exts[0], ".")
	}
	return "bin"
}

// deriveObjectKey produces a stable, tenant-scoped key. Schema:
//
//	{account_id}/{hotel_id|"no-hotel"}/{kind}/{uuid}.{ext}
//
// The "no-hotel" branch supports the onboarding wizard where the FE has an
// authenticated session but no hotel yet. Once a hotel exists the FE should
// always include it.
func deriveObjectKey(accountID uuid.UUID, hotelID *uuid.UUID, kind Kind, contentType string, fileID uuid.UUID) string {
	h := "no-hotel"
	if hotelID != nil && *hotelID != uuid.Nil {
		h = hotelID.String()
	}
	ext := extensionFor(contentType)
	return fmt.Sprintf("%s/%s/%s/%s.%s", accountID, h, kind, fileID, ext)
}

func (s *Service) publicURLFor(objectKey string) string {
	if s.publicBase == "" {
		return ""
	}
	return s.publicBase + "/" + objectKey
}

// MapError centralises sentinel→HTTP mapping for the handler. Exported so
// handler.go can stay slim. We keep this in service.go because the sentinels
// are package-private; tests in the same package use it directly.
func mapErrorCode(err error) (status int, code, msg string) {
	switch {
	case errors.Is(err, ErrUnsupportedKind):
		return 400, "UNSUPPORTED_KIND", "kind must be hotel_photo or room_type_photo"
	case errors.Is(err, ErrInvalidContentType):
		return 400, "INVALID_CONTENT_TYPE", "content_type must be one of " + strings.Join(AllowedContentTypes, ", ")
	case errors.Is(err, ErrSizeExceeded):
		return 400, "SIZE_EXCEEDED", fmt.Sprintf("size_bytes must be 1..%d", MaxUploadBytes)
	case errors.Is(err, ErrInvalidRequest):
		return 400, "BAD_REQUEST", "invalid request"
	case errors.Is(err, ErrNotConfigured):
		return 503, "STORAGE_UNAVAILABLE", "object storage is not configured on this server"
	case errors.Is(err, ErrHotelNotOwned):
		return 404, "NOT_FOUND", "hotel not found"
	}
	return 500, "INTERNAL", "internal error"
}
