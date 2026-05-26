package landing

import (
	"context"
	"regexp"
	"strings"

	"github.com/google/uuid"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// localeRegex matches BCP-47-ish two-letter codes with an optional uppercase
// region (e.g. "th", "en", "en-US"). The set Phase 1 supports is small (TH/EN)
// but we accept any shape that matches to avoid coupling validation to a
// hardcoded list.
var localeRegex = regexp.MustCompile(`^[a-z]{2}(-[A-Z]{2})?$`)

// colorRegex matches strict 6-digit hex colors. Branding stays simple — no
// shorthand (#fff), no rgba(), no named colors. The frontend is responsible
// for deriving lighter/darker shades for hover states.
var colorRegex = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)

// allowedSectionTypes is the Phase 1 section catalog. Anything outside this
// set is rejected so arbitrary JSON can't accumulate via the public API.
var allowedSectionTypes = map[string]struct{}{
	"hero":      {},
	"gallery":   {},
	"about":     {},
	"rooms":     {},
	"amenities": {},
	"location":  {},
	"reviews":   {},
	"faq":       {},
	"policies":  {},
	"contact":   {},
}

// PrimaryLocale returns the most recently published locale for a hotel's
// landing pages, or "th" when nothing has been published yet. Used by the
// booking event hook to pick a render language for the guest email.
func (s *Service) PrimaryLocale(ctx context.Context, hotelID uuid.UUID) (string, error) {
	loc, err := s.repo.PrimaryLocale(ctx, hotelID)
	if err != nil {
		return "th", err
	}
	if loc == "" {
		return "th", nil
	}
	return loc, nil
}

// ensureHotelOwned returns ErrHotelNotFound if the hotel doesn't exist or
// isn't owned by accountID. Centralizes tenant-isolation so every endpoint
// hits the same check.
func (s *Service) ensureHotelOwned(ctx context.Context, accountID, hotelID uuid.UUID) error {
	ok, err := s.repo.HotelBelongsToAccount(ctx, accountID, hotelID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrHotelNotFound
	}
	return nil
}

func (s *Service) List(ctx context.Context, accountID, hotelID uuid.UUID) ([]LandingPage, error) {
	if err := s.ensureHotelOwned(ctx, accountID, hotelID); err != nil {
		return nil, err
	}
	return s.repo.ListByHotel(ctx, hotelID)
}

func (s *Service) Get(ctx context.Context, accountID, hotelID uuid.UUID, locale string) (*LandingPage, error) {
	locale = strings.TrimSpace(locale)
	if err := validateLocale(locale); err != nil {
		return nil, err
	}
	if err := s.ensureHotelOwned(ctx, accountID, hotelID); err != nil {
		return nil, err
	}
	return s.repo.Get(ctx, hotelID, locale)
}

func (s *Service) Upsert(ctx context.Context, accountID, hotelID uuid.UUID, locale string, req UpdateRequest) (*LandingPage, error) {
	locale = strings.TrimSpace(locale)
	if err := validateLocale(locale); err != nil {
		return nil, err
	}
	if err := validateBranding(req.Branding); err != nil {
		return nil, err
	}
	if err := validateSections(req.Sections); err != nil {
		return nil, err
	}
	if err := s.ensureHotelOwned(ctx, accountID, hotelID); err != nil {
		return nil, err
	}
	return s.repo.Upsert(ctx, hotelID, locale, req.Branding, req.Sections, req.SEO, req.Tracking)
}

func (s *Service) Publish(ctx context.Context, accountID, hotelID uuid.UUID, locale string) (*LandingPage, error) {
	locale = strings.TrimSpace(locale)
	if err := validateLocale(locale); err != nil {
		return nil, err
	}
	if err := s.ensureHotelOwned(ctx, accountID, hotelID); err != nil {
		return nil, err
	}
	return s.repo.Publish(ctx, hotelID, locale)
}

func (s *Service) Unpublish(ctx context.Context, accountID, hotelID uuid.UUID, locale string) (*LandingPage, error) {
	locale = strings.TrimSpace(locale)
	if err := validateLocale(locale); err != nil {
		return nil, err
	}
	if err := s.ensureHotelOwned(ctx, accountID, hotelID); err != nil {
		return nil, err
	}
	return s.repo.Unpublish(ctx, hotelID, locale)
}

func (s *Service) Delete(ctx context.Context, accountID, hotelID uuid.UUID, locale string) error {
	locale = strings.TrimSpace(locale)
	if err := validateLocale(locale); err != nil {
		return err
	}
	if err := s.ensureHotelOwned(ctx, accountID, hotelID); err != nil {
		return err
	}
	return s.repo.Delete(ctx, hotelID, locale)
}

// GetPublishedBySlug is the public-facing read path used by the booking-web
// ISR build. It bypasses account ownership checks (it's anonymous) but only
// ever returns published rows on live hotels.
func (s *Service) GetPublishedBySlug(ctx context.Context, slug, locale string) (*PublicLandingResponse, error) {
	locale = strings.TrimSpace(locale)
	if err := validateLocale(locale); err != nil {
		return nil, err
	}
	slug = strings.TrimSpace(strings.ToLower(slug))
	if slug == "" {
		return nil, ErrLandingPageNotFound
	}
	return s.repo.GetPublishedBySlug(ctx, slug, locale)
}

func validateLocale(s string) error {
	if !localeRegex.MatchString(s) {
		return ErrInvalidLocale
	}
	return nil
}

func validateBranding(b Branding) error {
	if b.PrimaryColor != "" && !colorRegex.MatchString(b.PrimaryColor) {
		return ErrInvalidColor
	}
	if b.AccentColor != "" && !colorRegex.MatchString(b.AccentColor) {
		return ErrInvalidColor
	}
	return nil
}

func validateSections(sections []Section) error {
	seenOrder := make(map[int]struct{}, len(sections))
	for _, sec := range sections {
		if !isAllowedSectionType(sec.Type) {
			return ErrInvalidSectionType
		}
		if _, dup := seenOrder[sec.Order]; dup {
			return ErrInvalidSection
		}
		seenOrder[sec.Order] = struct{}{}
	}
	return nil
}

func isAllowedSectionType(t string) bool {
	_, ok := allowedSectionTypes[t]
	return ok
}
