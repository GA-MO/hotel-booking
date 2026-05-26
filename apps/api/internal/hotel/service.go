package hotel

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

var slugRegex = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

const (
	slugMinLen = 3
	slugMaxLen = 80
	nameMaxLen = 255
)

func (s *Service) Create(ctx context.Context, accountID uuid.UUID, req CreateRequest) (*Hotel, error) {
	slug := strings.TrimSpace(strings.ToLower(req.Slug))
	if err := validateSlug(slug); err != nil {
		return nil, err
	}
	name := strings.TrimSpace(req.Name)
	if err := validateName(name); err != nil {
		return nil, err
	}
	if req.PromptPayID != nil {
		normalised, err := normalisePromptPayID(*req.PromptPayID)
		if err != nil {
			return nil, err
		}
		req.PromptPayID = &normalised
	}
	return s.repo.Create(ctx, accountID, slug, name, req.HotelType, req.Country, req.Timezone, req.BaseCurrency, req.PromptPayID)
}

func (s *Service) Get(ctx context.Context, accountID, id uuid.UUID) (*Hotel, error) {
	return s.repo.GetByID(ctx, accountID, id)
}

func (s *Service) List(ctx context.Context, accountID uuid.UUID) ([]Hotel, error) {
	return s.repo.ListByAccount(ctx, accountID)
}

func (s *Service) Update(ctx context.Context, accountID, id uuid.UUID, req UpdateRequest) (*Hotel, error) {
	if req.Name != nil {
		trimmed := strings.TrimSpace(*req.Name)
		if err := validateName(trimmed); err != nil {
			return nil, err
		}
		req.Name = &trimmed
	}
	if req.PromptPayID != nil {
		// Empty string is allowed as the "clear it" signal — service stores
		// NULL in that case; only non-empty values are format-validated.
		raw := strings.TrimSpace(*req.PromptPayID)
		if raw == "" {
			empty := ""
			req.PromptPayID = &empty
		} else {
			normalised, err := normalisePromptPayID(raw)
			if err != nil {
				return nil, err
			}
			req.PromptPayID = &normalised
		}
	}
	return s.repo.Update(ctx, accountID, id, req)
}

func (s *Service) Delete(ctx context.Context, accountID, id uuid.UUID) error {
	return s.repo.SoftDelete(ctx, accountID, id)
}

func (s *Service) SlugAvailable(ctx context.Context, slug string) (bool, error) {
	slug = strings.TrimSpace(strings.ToLower(slug))
	if err := validateSlug(slug); err != nil {
		return false, err
	}
	return s.repo.SlugAvailable(ctx, slug)
}

func validateSlug(s string) error {
	if len(s) < slugMinLen || len(s) > slugMaxLen {
		return ErrInvalidSlug
	}
	if !slugRegex.MatchString(s) {
		return ErrInvalidSlug
	}
	return nil
}

func validateName(s string) error {
	if s == "" || len(s) > nameMaxLen {
		return ErrInvalidName
	}
	return nil
}

// promptPayStripRegex matches dashes and ASCII whitespace, which Thai banking
// UIs commonly insert into PromptPay IDs (e.g. "081-234-5678" or
// "0-1234-56789-01-1"). We strip them before validating digit-count.
var promptPayStripRegex = regexp.MustCompile(`[\s-]+`)

// promptPayDigitsOnly matches strings that are all digits.
var promptPayDigitsOnly = regexp.MustCompile(`^[0-9]+$`)

// promptPayTaxIDPattern matches a 15-character e-Wallet / corporate tax ID
// PromptPay receiver: a leading "0" prefix followed by 14 digits. The "0"
// prefix distinguishes a 15-char merchant ID from a 13-digit national ID.
// (Bank of Thailand EMVCo tag-29/30 conventions.)
var promptPayTaxIDPattern = regexp.MustCompile(`^0[0-9]{14}$`)

// normalisePromptPayID strips dashes and whitespace, then validates against
// the three PromptPay receiver formats: 10-digit phone (mobile MSISDN
// without country code), 13-digit Thai national-ID, or 15-char e-Wallet /
// tax-ID. Returns the cleaned value on success.
func normalisePromptPayID(in string) (string, error) {
	cleaned := promptPayStripRegex.ReplaceAllString(strings.TrimSpace(in), "")
	switch len(cleaned) {
	case 10, 13:
		if !promptPayDigitsOnly.MatchString(cleaned) {
			return "", ErrInvalidPromptPayID
		}
		return cleaned, nil
	case 15:
		if !promptPayTaxIDPattern.MatchString(cleaned) {
			return "", ErrInvalidPromptPayID
		}
		return cleaned, nil
	default:
		return "", ErrInvalidPromptPayID
	}
}
