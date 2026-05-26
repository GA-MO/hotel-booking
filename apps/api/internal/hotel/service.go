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
	return s.repo.Create(ctx, accountID, slug, name, req.HotelType, req.Country, req.Timezone, req.BaseCurrency)
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
