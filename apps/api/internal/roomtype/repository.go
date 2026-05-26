package roomtype

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/GA-MO/hotel-booking/apps/api/internal/platform/dberr"
)

var isUniqueViolation = dberr.IsUniqueViolation

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// ----- room_types -----

const selectColumns = `
	rt.id, rt.hotel_id,
	rt.name, COALESCE(rt.description, ''),
	rt.total_inventory, rt.max_occupancy,
	rt.size_sqm,
	rt.bed_config, rt.amenities,
	rt.base_rate, rt.base_currency,
	rt.display_order, rt.enabled,
	rt.created_at, rt.updated_at
`

// Unaliased version used for INSERT ... RETURNING (PostgreSQL doesn't accept
// arbitrary aliases on the inserted-into table — only the actual table name).
const returningColumns = `
	id, hotel_id,
	name, COALESCE(description, ''),
	total_inventory, max_occupancy,
	size_sqm,
	bed_config, amenities,
	base_rate, base_currency,
	display_order, enabled,
	created_at, updated_at
`

func scanRoomType(row pgx.Row) (*RoomType, error) {
	var rt RoomType
	var bedConfig, amenities []byte
	if err := row.Scan(
		&rt.ID, &rt.HotelID,
		&rt.Name, &rt.Description,
		&rt.TotalInventory, &rt.MaxOccupancy,
		&rt.SizeSqm,
		&bedConfig, &amenities,
		&rt.BaseRate, &rt.BaseCurrency,
		&rt.DisplayOrder, &rt.Enabled,
		&rt.CreatedAt, &rt.UpdatedAt,
	); err != nil {
		return nil, err
	}
	rt.BedConfig = json.RawMessage(bedConfig)
	rt.Amenities = json.RawMessage(amenities)
	return &rt, nil
}

// hotelOwnedByAccount returns true if the hotel exists, is not soft-deleted,
// and belongs to the caller's account. Cross-tenant access is treated as
// not-found by callers (avoid leaking existence).
func (r *Repository) hotelOwnedByAccount(ctx context.Context, accountID, hotelID uuid.UUID) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM hotels
			WHERE id = $1 AND account_id = $2 AND deleted_at IS NULL
		)
	`, hotelID, accountID).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}

func (r *Repository) Create(
	ctx context.Context,
	accountID, hotelID uuid.UUID,
	req CreateRequest,
) (*RoomType, error) {
	ok, err := r.hotelOwnedByAccount(ctx, accountID, hotelID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrRoomTypeNotFound
	}

	bedConfig := normalizeJSON(req.BedConfig, []byte(`{}`))
	amenities := normalizeJSON(req.Amenities, []byte(`[]`))
	baseCurrency := req.BaseCurrency
	if baseCurrency == "" {
		baseCurrency = "THB"
	}

	row := r.db.QueryRow(ctx, `
		INSERT INTO room_types (
			hotel_id, name, description,
			total_inventory, max_occupancy, size_sqm,
			bed_config, amenities,
			base_rate, base_currency, display_order
		)
		VALUES ($1, $2, NULLIF($3, ''),
		        $4, $5, $6,
		        $7::jsonb, $8::jsonb,
		        $9, $10, $11)
		RETURNING `+returningColumns+`
	`, hotelID, req.Name, req.Description,
		req.TotalInventory, req.MaxOccupancy, req.SizeSqm,
		bedConfig, amenities,
		req.BaseRate, baseCurrency, req.DisplayOrder,
	)
	rt, err := scanRoomType(row)
	if err != nil {
		return nil, fmt.Errorf("insert room_type: %w", err)
	}
	return rt, nil
}

func (r *Repository) GetByID(ctx context.Context, accountID, hotelID, id uuid.UUID) (*RoomType, error) {
	row := r.db.QueryRow(ctx, `
		SELECT `+selectColumns+`
		FROM room_types rt
		JOIN hotels h ON h.id = rt.hotel_id
		WHERE rt.id = $1
		  AND rt.hotel_id = $2
		  AND h.account_id = $3
		  AND rt.deleted_at IS NULL
		  AND h.deleted_at IS NULL
	`, id, hotelID, accountID)
	rt, err := scanRoomType(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrRoomTypeNotFound
		}
		return nil, err
	}
	return rt, nil
}

func (r *Repository) ListByHotel(ctx context.Context, accountID, hotelID uuid.UUID) ([]RoomType, error) {
	ok, err := r.hotelOwnedByAccount(ctx, accountID, hotelID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrRoomTypeNotFound
	}
	rows, err := r.db.Query(ctx, `
		SELECT `+selectColumns+`
		FROM room_types rt
		WHERE rt.hotel_id = $1 AND rt.deleted_at IS NULL
		ORDER BY rt.display_order ASC, rt.created_at ASC
	`, hotelID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []RoomType{}
	for rows.Next() {
		rt, err := scanRoomType(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, *rt)
	}
	return result, rows.Err()
}

func (r *Repository) Update(ctx context.Context, accountID, hotelID, id uuid.UUID, patch UpdateRequest) (*RoomType, error) {
	// Ensure ownership and existence in one round-trip via the WHERE clause on
	// the UPDATE; we re-SELECT with the same join to apply the account scope.
	tag, err := r.db.Exec(ctx, `
		UPDATE room_types rt SET
		  name            = COALESCE($4,  rt.name),
		  description     = COALESCE($5,  rt.description),
		  total_inventory = COALESCE($6,  rt.total_inventory),
		  max_occupancy   = COALESCE($7,  rt.max_occupancy),
		  size_sqm        = COALESCE($8,  rt.size_sqm),
		  bed_config      = COALESCE($9::jsonb,  rt.bed_config),
		  amenities       = COALESCE($10::jsonb, rt.amenities),
		  base_rate       = COALESCE($11, rt.base_rate),
		  base_currency   = COALESCE($12, rt.base_currency),
		  display_order   = COALESCE($13, rt.display_order),
		  enabled         = COALESCE($14, rt.enabled)
		FROM hotels h
		WHERE rt.id = $1
		  AND rt.hotel_id = $2
		  AND h.id = rt.hotel_id
		  AND h.account_id = $3
		  AND rt.deleted_at IS NULL
		  AND h.deleted_at IS NULL
	`,
		id, hotelID, accountID,
		ptrOrNil(patch.Name),
		ptrOrNil(patch.Description),
		ptrOrNil(patch.TotalInventory),
		ptrOrNil(patch.MaxOccupancy),
		ptrOrNil(patch.SizeSqm),
		rawMessagePtrOrNil(patch.BedConfig),
		rawMessagePtrOrNil(patch.Amenities),
		ptrOrNil(patch.BaseRate),
		ptrOrNil(patch.BaseCurrency),
		ptrOrNil(patch.DisplayOrder),
		ptrOrNil(patch.Enabled),
	)
	if err != nil {
		return nil, fmt.Errorf("update room_type: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return nil, ErrRoomTypeNotFound
	}
	return r.GetByID(ctx, accountID, hotelID, id)
}

func (r *Repository) SoftDelete(ctx context.Context, accountID, hotelID, id uuid.UUID) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE room_types rt SET deleted_at = NOW()
		FROM hotels h
		WHERE rt.id = $1
		  AND rt.hotel_id = $2
		  AND h.id = rt.hotel_id
		  AND h.account_id = $3
		  AND rt.deleted_at IS NULL
		  AND h.deleted_at IS NULL
	`, id, hotelID, accountID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrRoomTypeNotFound
	}
	return nil
}

// ----- room_type_photos -----

const photoColumnsRoomType = `
	p.id, NULL::uuid AS hotel_id, p.room_type_id,
	p.storage_key, COALESCE(p.caption, ''), COALESCE(p.alt_text, ''),
	p.width, p.height,
	p.display_order, p.is_cover, p.created_at
`

const photoColumnsHotel = `
	p.id, p.hotel_id, NULL::uuid AS room_type_id,
	p.storage_key, COALESCE(p.caption, ''), COALESCE(p.alt_text, ''),
	p.width, p.height,
	p.display_order, p.is_cover, p.created_at
`

// INSERT ... RETURNING variants (no alias, see selectColumns / returningColumns).
const returningPhotoRoomType = `
	id, NULL::uuid AS hotel_id, room_type_id,
	storage_key, COALESCE(caption, ''), COALESCE(alt_text, ''),
	width, height,
	display_order, is_cover, created_at
`

const returningPhotoHotel = `
	id, hotel_id, NULL::uuid AS room_type_id,
	storage_key, COALESCE(caption, ''), COALESCE(alt_text, ''),
	width, height,
	display_order, is_cover, created_at
`

func scanPhoto(row pgx.Row) (*Photo, error) {
	var p Photo
	if err := row.Scan(
		&p.ID, &p.HotelID, &p.RoomTypeID,
		&p.StorageKey, &p.Caption, &p.AltText,
		&p.Width, &p.Height,
		&p.DisplayOrder, &p.IsCover, &p.CreatedAt,
	); err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *Repository) ListRoomTypePhotos(ctx context.Context, accountID, hotelID, roomTypeID uuid.UUID) ([]Photo, error) {
	// Verify ownership via the same join used by GetByID; missing room type =>
	// not found (regardless of whether it actually exists for another tenant).
	if _, err := r.GetByID(ctx, accountID, hotelID, roomTypeID); err != nil {
		return nil, err
	}
	rows, err := r.db.Query(ctx, `
		SELECT `+photoColumnsRoomType+`
		FROM room_type_photos p
		WHERE p.room_type_id = $1
		ORDER BY p.display_order ASC, p.created_at ASC
	`, roomTypeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []Photo{}
	for rows.Next() {
		ph, err := scanPhoto(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, *ph)
	}
	return result, rows.Err()
}

func (r *Repository) CreateRoomTypePhoto(ctx context.Context, accountID, hotelID, roomTypeID uuid.UUID, req CreatePhotoRequest) (*Photo, error) {
	if _, err := r.GetByID(ctx, accountID, hotelID, roomTypeID); err != nil {
		return nil, err
	}
	row := r.db.QueryRow(ctx, `
		INSERT INTO room_type_photos (
			room_type_id, storage_key, caption, alt_text,
			width, height, display_order, is_cover
		)
		VALUES ($1, $2, NULLIF($3, ''), NULLIF($4, ''),
		        $5, $6, $7, $8)
		RETURNING `+returningPhotoRoomType+`
	`,
		roomTypeID, req.StorageKey, req.Caption, req.AltText,
		req.Width, req.Height, req.DisplayOrder, req.IsCover,
	)
	p, err := scanPhoto(row)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrCoverConflict
		}
		return nil, fmt.Errorf("insert room_type_photo: %w", err)
	}
	return p, nil
}

func (r *Repository) UpdateRoomTypePhoto(ctx context.Context, accountID, hotelID, roomTypeID, photoID uuid.UUID, patch UpdatePhotoRequest) (*Photo, error) {
	if _, err := r.GetByID(ctx, accountID, hotelID, roomTypeID); err != nil {
		return nil, err
	}
	tag, err := r.db.Exec(ctx, `
		UPDATE room_type_photos SET
		  caption       = COALESCE($3, caption),
		  alt_text      = COALESCE($4, alt_text),
		  display_order = COALESCE($5, display_order),
		  is_cover      = COALESCE($6, is_cover)
		WHERE id = $1 AND room_type_id = $2
	`,
		photoID, roomTypeID,
		ptrOrNil(patch.Caption),
		ptrOrNil(patch.AltText),
		ptrOrNil(patch.DisplayOrder),
		ptrOrNil(patch.IsCover),
	)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrCoverConflict
		}
		return nil, fmt.Errorf("update room_type_photo: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return nil, ErrPhotoNotFound
	}
	row := r.db.QueryRow(ctx, `
		SELECT `+photoColumnsRoomType+`
		FROM room_type_photos p
		WHERE p.id = $1 AND p.room_type_id = $2
	`, photoID, roomTypeID)
	return scanPhoto(row)
}

func (r *Repository) DeleteRoomTypePhoto(ctx context.Context, accountID, hotelID, roomTypeID, photoID uuid.UUID) error {
	if _, err := r.GetByID(ctx, accountID, hotelID, roomTypeID); err != nil {
		return err
	}
	tag, err := r.db.Exec(ctx, `
		DELETE FROM room_type_photos WHERE id = $1 AND room_type_id = $2
	`, photoID, roomTypeID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrPhotoNotFound
	}
	return nil
}

// ----- hotel_photos -----

func (r *Repository) ListHotelPhotos(ctx context.Context, accountID, hotelID uuid.UUID) ([]Photo, error) {
	ok, err := r.hotelOwnedByAccount(ctx, accountID, hotelID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrRoomTypeNotFound
	}
	rows, err := r.db.Query(ctx, `
		SELECT `+photoColumnsHotel+`
		FROM hotel_photos p
		WHERE p.hotel_id = $1
		ORDER BY p.display_order ASC, p.created_at ASC
	`, hotelID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []Photo{}
	for rows.Next() {
		ph, err := scanPhoto(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, *ph)
	}
	return result, rows.Err()
}

func (r *Repository) CreateHotelPhoto(ctx context.Context, accountID, hotelID uuid.UUID, req CreatePhotoRequest) (*Photo, error) {
	ok, err := r.hotelOwnedByAccount(ctx, accountID, hotelID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrRoomTypeNotFound
	}
	row := r.db.QueryRow(ctx, `
		INSERT INTO hotel_photos (
			hotel_id, storage_key, caption, alt_text,
			width, height, display_order, is_cover
		)
		VALUES ($1, $2, NULLIF($3, ''), NULLIF($4, ''),
		        $5, $6, $7, $8)
		RETURNING `+returningPhotoHotel+`
	`,
		hotelID, req.StorageKey, req.Caption, req.AltText,
		req.Width, req.Height, req.DisplayOrder, req.IsCover,
	)
	p, err := scanPhoto(row)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrCoverConflict
		}
		return nil, fmt.Errorf("insert hotel_photo: %w", err)
	}
	return p, nil
}

func (r *Repository) DeleteHotelPhoto(ctx context.Context, accountID, hotelID, photoID uuid.UUID) error {
	ok, err := r.hotelOwnedByAccount(ctx, accountID, hotelID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrPhotoNotFound
	}
	tag, err := r.db.Exec(ctx, `
		DELETE FROM hotel_photos WHERE id = $1 AND hotel_id = $2
	`, photoID, hotelID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrPhotoNotFound
	}
	return nil
}

// ----- helpers -----

func normalizeJSON(raw json.RawMessage, fallback []byte) []byte {
	if len(raw) == 0 {
		return fallback
	}
	return []byte(raw)
}

func rawMessagePtrOrNil(p *json.RawMessage) any {
	if p == nil {
		return nil
	}
	if len(*p) == 0 {
		return nil
	}
	return []byte(*p)
}

func ptrOrNil[T any](p *T) any {
	if p == nil {
		return nil
	}
	return *p
}

