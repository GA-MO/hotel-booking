package hotel

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

const selectColumns = `
	id, account_id, slug, name,
	COALESCE(hotel_type, ''), COALESCE(description, ''),
	COALESCE(address_line, ''), COALESCE(city, ''),
	COALESCE(country, ''), COALESCE(postal_code, ''),
	latitude, longitude,
	COALESCE(phone, ''), COALESCE(email, ''), COALESCE(line_id, ''),
	timezone, base_currency,
	to_char(check_in_time, 'HH24:MI:SS'),
	to_char(check_out_time, 'HH24:MI:SS'),
	kyc_status, status,
	promptpay_id,
	created_at, updated_at
`

func scanHotel(row pgx.Row) (*Hotel, error) {
	var h Hotel
	if err := row.Scan(
		&h.ID, &h.AccountID, &h.Slug, &h.Name,
		&h.HotelType, &h.Description,
		&h.AddressLine, &h.City,
		&h.Country, &h.PostalCode,
		&h.Latitude, &h.Longitude,
		&h.Phone, &h.Email, &h.LineID,
		&h.Timezone, &h.BaseCurrency,
		&h.CheckInTime, &h.CheckOutTime,
		&h.KYCStatus, &h.Status,
		&h.PromptPayID,
		&h.CreatedAt, &h.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &h, nil
}

func (r *Repository) Create(
	ctx context.Context,
	accountID uuid.UUID,
	slug, name, hotelType, country, timezone, baseCurrency string,
	promptPayID *string,
) (*Hotel, error) {
	if timezone == "" {
		timezone = "Asia/Bangkok"
	}
	if baseCurrency == "" {
		baseCurrency = "THB"
	}
	row := r.db.QueryRow(ctx, `
		INSERT INTO hotels (account_id, slug, name, hotel_type, country, timezone, base_currency, promptpay_id)
		VALUES ($1, $2, $3, NULLIF($4, ''), NULLIF($5, ''), $6, $7, $8)
		RETURNING `+selectColumns,
		accountID, slug, name, hotelType, country, timezone, baseCurrency, ptrOrNil(promptPayID),
	)
	h, err := scanHotel(row)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrSlugAlreadyTaken
		}
		return nil, fmt.Errorf("insert hotel: %w", err)
	}
	return h, nil
}

func (r *Repository) GetByID(ctx context.Context, accountID, id uuid.UUID) (*Hotel, error) {
	row := r.db.QueryRow(ctx, `
		SELECT `+selectColumns+`
		FROM hotels
		WHERE id = $1 AND account_id = $2 AND deleted_at IS NULL
	`, id, accountID)
	h, err := scanHotel(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrHotelNotFound
		}
		return nil, err
	}
	return h, nil
}

func (r *Repository) ListByAccount(ctx context.Context, accountID uuid.UUID) ([]Hotel, error) {
	rows, err := r.db.Query(ctx, `
		SELECT `+selectColumns+`
		FROM hotels
		WHERE account_id = $1 AND deleted_at IS NULL
		ORDER BY created_at DESC
	`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	hotels := []Hotel{}
	for rows.Next() {
		h, err := scanHotel(rows)
		if err != nil {
			return nil, err
		}
		hotels = append(hotels, *h)
	}
	return hotels, rows.Err()
}

// SlugAvailable returns true if no hotel currently owns the slug.
// Note: only an advisory check — final ownership is decided by the unique
// constraint at INSERT time, since other tenants may claim the slug between
// check and create.
func (r *Repository) SlugAvailable(ctx context.Context, slug string) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM hotels WHERE slug = $1)
	`, slug).Scan(&exists)
	if err != nil {
		return false, err
	}
	return !exists, nil
}

// Update applies a partial update. Nil pointers in patch are left unchanged.
// PromptPayID has tri-state semantics: nil = unchanged, "" = clear (write
// NULL), non-empty string = write the new value.
func (r *Repository) Update(ctx context.Context, accountID, id uuid.UUID, patch UpdateRequest) (*Hotel, error) {
	// Encode tri-state for promptpay_id. We use a sentinel boolean so the
	// SQL knows whether the caller intended to touch the column at all.
	var promptPay any
	clearPromptPay := false
	if patch.PromptPayID != nil {
		if *patch.PromptPayID == "" {
			clearPromptPay = true
		} else {
			v := *patch.PromptPayID
			promptPay = v
		}
	}
	row := r.db.QueryRow(ctx, `
		UPDATE hotels SET
		  name           = COALESCE($3,  name),
		  hotel_type     = COALESCE($4,  hotel_type),
		  description    = COALESCE($5,  description),
		  address_line   = COALESCE($6,  address_line),
		  city           = COALESCE($7,  city),
		  country        = COALESCE($8,  country),
		  postal_code    = COALESCE($9,  postal_code),
		  latitude       = COALESCE($10, latitude),
		  longitude      = COALESCE($11, longitude),
		  phone          = COALESCE($12, phone),
		  email          = COALESCE($13, email),
		  line_id        = COALESCE($14, line_id),
		  timezone       = COALESCE($15, timezone),
		  base_currency  = COALESCE($16, base_currency),
		  check_in_time  = COALESCE($17::time, check_in_time),
		  check_out_time = COALESCE($18::time, check_out_time),
		  promptpay_id   = CASE
		                     WHEN $20::boolean THEN NULL
		                     ELSE COALESCE($19, promptpay_id)
		                   END
		WHERE id = $1 AND account_id = $2 AND deleted_at IS NULL
		RETURNING `+selectColumns,
		id, accountID,
		ptrOrNil(patch.Name),
		ptrOrNil(patch.HotelType),
		ptrOrNil(patch.Description),
		ptrOrNil(patch.AddressLine),
		ptrOrNil(patch.City),
		ptrOrNil(patch.Country),
		ptrOrNil(patch.PostalCode),
		ptrOrNil(patch.Latitude),
		ptrOrNil(patch.Longitude),
		ptrOrNil(patch.Phone),
		ptrOrNil(patch.Email),
		ptrOrNil(patch.LineID),
		ptrOrNil(patch.Timezone),
		ptrOrNil(patch.BaseCurrency),
		ptrOrNil(patch.CheckInTime),
		ptrOrNil(patch.CheckOutTime),
		promptPay,
		clearPromptPay,
	)
	h, err := scanHotel(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrHotelNotFound
		}
		return nil, err
	}
	return h, nil
}

// SetStatus flips hotel.status to one of test|live|suspended|archived,
// scoped to the caller's account. Returns the refreshed Hotel. The state
// machine itself (which transitions are legal) lives in the service.
func (r *Repository) SetStatus(ctx context.Context, accountID, id uuid.UUID, status string) (*Hotel, error) {
	row := r.db.QueryRow(ctx, `
		UPDATE hotels SET status = $3
		WHERE id = $1 AND account_id = $2 AND deleted_at IS NULL
		RETURNING `+selectColumns,
		id, accountID, status,
	)
	h, err := scanHotel(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrHotelNotFound
		}
		return nil, err
	}
	return h, nil
}

func (r *Repository) SoftDelete(ctx context.Context, accountID, id uuid.UUID) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE hotels SET deleted_at = NOW()
		WHERE id = $1 AND account_id = $2 AND deleted_at IS NULL
	`, id, accountID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrHotelNotFound
	}
	return nil
}

func ptrOrNil[T any](p *T) any {
	if p == nil {
		return nil
	}
	return *p
}

func isUniqueViolation(err error) bool {
	const code = "23505"
	type pgErr interface{ SQLState() string }
	var pe pgErr
	if errors.As(err, &pe) {
		return pe.SQLState() == code
	}
	return false
}
