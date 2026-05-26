package roomtype

import (
	"context"
	"strings"

	"github.com/google/uuid"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

const (
	nameMinLen     = 1
	nameMaxLen     = 120
	currencyLen    = 3
	storageKeyMax  = 1024
)

func (s *Service) Create(ctx context.Context, accountID, hotelID uuid.UUID, req CreateRequest) (*RoomType, error) {
	req.Name = strings.TrimSpace(req.Name)
	req.Description = strings.TrimSpace(req.Description)
	if err := validateName(req.Name); err != nil {
		return nil, err
	}
	if err := validateCapacity(req.MaxOccupancy); err != nil {
		return nil, err
	}
	if err := validateInventory(req.TotalInventory); err != nil {
		return nil, err
	}
	if err := validateBaseRate(req.BaseRate); err != nil {
		return nil, err
	}
	if req.BaseCurrency != "" {
		if err := validateCurrency(req.BaseCurrency); err != nil {
			return nil, err
		}
		req.BaseCurrency = strings.ToUpper(req.BaseCurrency)
	}
	if err := validateDisplayOrder(req.DisplayOrder); err != nil {
		return nil, err
	}
	return s.repo.Create(ctx, accountID, hotelID, req)
}

func (s *Service) Get(ctx context.Context, accountID, hotelID, id uuid.UUID) (*RoomType, error) {
	return s.repo.GetByID(ctx, accountID, hotelID, id)
}

func (s *Service) List(ctx context.Context, accountID, hotelID uuid.UUID) ([]RoomType, error) {
	return s.repo.ListByHotel(ctx, accountID, hotelID)
}

func (s *Service) Update(ctx context.Context, accountID, hotelID, id uuid.UUID, req UpdateRequest) (*RoomType, error) {
	if req.Name != nil {
		trimmed := strings.TrimSpace(*req.Name)
		if err := validateName(trimmed); err != nil {
			return nil, err
		}
		req.Name = &trimmed
	}
	if req.MaxOccupancy != nil {
		if err := validateCapacity(*req.MaxOccupancy); err != nil {
			return nil, err
		}
	}
	if req.TotalInventory != nil {
		if err := validateInventory(*req.TotalInventory); err != nil {
			return nil, err
		}
	}
	if req.BaseRate != nil {
		if err := validateBaseRate(*req.BaseRate); err != nil {
			return nil, err
		}
	}
	if req.BaseCurrency != nil {
		if err := validateCurrency(*req.BaseCurrency); err != nil {
			return nil, err
		}
		upper := strings.ToUpper(*req.BaseCurrency)
		req.BaseCurrency = &upper
	}
	if req.DisplayOrder != nil {
		if err := validateDisplayOrder(*req.DisplayOrder); err != nil {
			return nil, err
		}
	}
	return s.repo.Update(ctx, accountID, hotelID, id, req)
}

func (s *Service) Delete(ctx context.Context, accountID, hotelID, id uuid.UUID) error {
	return s.repo.SoftDelete(ctx, accountID, hotelID, id)
}

// ----- photos -----

func (s *Service) ListRoomTypePhotos(ctx context.Context, accountID, hotelID, roomTypeID uuid.UUID) ([]Photo, error) {
	return s.repo.ListRoomTypePhotos(ctx, accountID, hotelID, roomTypeID)
}

func (s *Service) CreateRoomTypePhoto(ctx context.Context, accountID, hotelID, roomTypeID uuid.UUID, req CreatePhotoRequest) (*Photo, error) {
	if err := validatePhotoCreate(&req); err != nil {
		return nil, err
	}
	return s.repo.CreateRoomTypePhoto(ctx, accountID, hotelID, roomTypeID, req)
}

func (s *Service) UpdateRoomTypePhoto(ctx context.Context, accountID, hotelID, roomTypeID, photoID uuid.UUID, req UpdatePhotoRequest) (*Photo, error) {
	if err := validatePhotoUpdate(&req); err != nil {
		return nil, err
	}
	return s.repo.UpdateRoomTypePhoto(ctx, accountID, hotelID, roomTypeID, photoID, req)
}

func (s *Service) DeleteRoomTypePhoto(ctx context.Context, accountID, hotelID, roomTypeID, photoID uuid.UUID) error {
	return s.repo.DeleteRoomTypePhoto(ctx, accountID, hotelID, roomTypeID, photoID)
}

func (s *Service) ListHotelPhotos(ctx context.Context, accountID, hotelID uuid.UUID) ([]Photo, error) {
	return s.repo.ListHotelPhotos(ctx, accountID, hotelID)
}

func (s *Service) CreateHotelPhoto(ctx context.Context, accountID, hotelID uuid.UUID, req CreatePhotoRequest) (*Photo, error) {
	if err := validatePhotoCreate(&req); err != nil {
		return nil, err
	}
	return s.repo.CreateHotelPhoto(ctx, accountID, hotelID, req)
}

func (s *Service) DeleteHotelPhoto(ctx context.Context, accountID, hotelID, photoID uuid.UUID) error {
	return s.repo.DeleteHotelPhoto(ctx, accountID, hotelID, photoID)
}

// ----- validation -----

func validateName(s string) error {
	if len(s) < nameMinLen || len(s) > nameMaxLen {
		return ErrInvalidName
	}
	return nil
}

func validateCapacity(n int) error {
	if n < 1 {
		return ErrInvalidCapacity
	}
	return nil
}

func validateInventory(n int) error {
	if n < 0 {
		return ErrInvalidInventory
	}
	return nil
}

func validateBaseRate(v float64) error {
	if v < 0 {
		return ErrInvalidBaseRate
	}
	return nil
}

func validateCurrency(s string) error {
	if len(s) != currencyLen {
		return ErrInvalidCurrency
	}
	for _, c := range s {
		if !(c >= 'A' && c <= 'Z') && !(c >= 'a' && c <= 'z') {
			return ErrInvalidCurrency
		}
	}
	return nil
}

func validateDisplayOrder(n int) error {
	if n < 0 {
		return ErrInvalidDisplayOrder
	}
	return nil
}

func validateStorageKey(s string) error {
	s = strings.TrimSpace(s)
	if s == "" || len(s) > storageKeyMax {
		return ErrInvalidStorageKey
	}
	return nil
}

func validatePhotoCreate(req *CreatePhotoRequest) error {
	req.StorageKey = strings.TrimSpace(req.StorageKey)
	if err := validateStorageKey(req.StorageKey); err != nil {
		return err
	}
	if err := validateDisplayOrder(req.DisplayOrder); err != nil {
		return err
	}
	return nil
}

func validatePhotoUpdate(req *UpdatePhotoRequest) error {
	if req.DisplayOrder != nil {
		if err := validateDisplayOrder(*req.DisplayOrder); err != nil {
			return err
		}
	}
	return nil
}
