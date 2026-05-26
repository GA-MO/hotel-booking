package landing

import (
	"context"
	"encoding/json"
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
	id, hotel_id, locale, status, version,
	branding, sections, seo, tracking,
	published_at, created_at, updated_at
`

// scanLandingPage scans one row into a LandingPage. JSONB columns are read as
// []byte and decoded into the concrete Go types — pgx exposes JSONB as raw
// bytes when scanned into []byte.
func scanLandingPage(row pgx.Row) (*LandingPage, error) {
	var (
		lp                                          LandingPage
		brandingRaw, sectionsRaw, seoRaw, trackRaw []byte
	)
	if err := row.Scan(
		&lp.ID, &lp.HotelID, &lp.Locale, &lp.Status, &lp.Version,
		&brandingRaw, &sectionsRaw, &seoRaw, &trackRaw,
		&lp.PublishedAt, &lp.CreatedAt, &lp.UpdatedAt,
	); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(brandingRaw, &lp.Branding); err != nil {
		return nil, fmt.Errorf("decode branding: %w", err)
	}
	// sections defaults to []; nil-safe decode preserves an empty array on the wire.
	lp.Sections = []Section{}
	if len(sectionsRaw) > 0 {
		if err := json.Unmarshal(sectionsRaw, &lp.Sections); err != nil {
			return nil, fmt.Errorf("decode sections: %w", err)
		}
		if lp.Sections == nil {
			lp.Sections = []Section{}
		}
	}
	if err := json.Unmarshal(seoRaw, &lp.SEO); err != nil {
		return nil, fmt.Errorf("decode seo: %w", err)
	}
	if err := json.Unmarshal(trackRaw, &lp.Tracking); err != nil {
		return nil, fmt.Errorf("decode tracking: %w", err)
	}
	return &lp, nil
}

// HotelBelongsToAccount returns true if the hotel exists, isn't deleted, and
// is owned by accountID. Used by the service to enforce tenant isolation
// before any landing-page operation runs.
func (r *Repository) HotelBelongsToAccount(ctx context.Context, accountID, hotelID uuid.UUID) (bool, error) {
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

func (r *Repository) Get(ctx context.Context, hotelID uuid.UUID, locale string) (*LandingPage, error) {
	row := r.db.QueryRow(ctx, `
		SELECT `+selectColumns+`
		FROM landing_pages
		WHERE hotel_id = $1 AND locale = $2
	`, hotelID, locale)
	lp, err := scanLandingPage(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrLandingPageNotFound
		}
		return nil, err
	}
	return lp, nil
}

func (r *Repository) ListByHotel(ctx context.Context, hotelID uuid.UUID) ([]LandingPage, error) {
	rows, err := r.db.Query(ctx, `
		SELECT `+selectColumns+`
		FROM landing_pages
		WHERE hotel_id = $1
		ORDER BY locale ASC
	`, hotelID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	pages := []LandingPage{}
	for rows.Next() {
		lp, err := scanLandingPage(rows)
		if err != nil {
			return nil, err
		}
		pages = append(pages, *lp)
	}
	return pages, rows.Err()
}

// Upsert creates a draft page if absent, or replaces branding/sections/seo/
// tracking and bumps version if present. Status is forced back to draft on
// every write — explicit publish is required to make it public.
func (r *Repository) Upsert(
	ctx context.Context,
	hotelID uuid.UUID,
	locale string,
	branding Branding,
	sections []Section,
	seo SEO,
	tracking Tracking,
) (*LandingPage, error) {
	if sections == nil {
		sections = []Section{}
	}
	brandingJSON, err := json.Marshal(branding)
	if err != nil {
		return nil, fmt.Errorf("encode branding: %w", err)
	}
	sectionsJSON, err := json.Marshal(sections)
	if err != nil {
		return nil, fmt.Errorf("encode sections: %w", err)
	}
	seoJSON, err := json.Marshal(seo)
	if err != nil {
		return nil, fmt.Errorf("encode seo: %w", err)
	}
	trackJSON, err := json.Marshal(tracking)
	if err != nil {
		return nil, fmt.Errorf("encode tracking: %w", err)
	}

	row := r.db.QueryRow(ctx, `
		INSERT INTO landing_pages (hotel_id, locale, branding, sections, seo, tracking)
		VALUES ($1, $2, $3::jsonb, $4::jsonb, $5::jsonb, $6::jsonb)
		ON CONFLICT (hotel_id, locale) DO UPDATE SET
			branding = EXCLUDED.branding,
			sections = EXCLUDED.sections,
			seo      = EXCLUDED.seo,
			tracking = EXCLUDED.tracking,
			version  = landing_pages.version + 1,
			status   = 'draft'
		RETURNING `+selectColumns,
		hotelID, locale, brandingJSON, sectionsJSON, seoJSON, trackJSON,
	)
	lp, err := scanLandingPage(row)
	if err != nil {
		return nil, fmt.Errorf("upsert landing page: %w", err)
	}
	return lp, nil
}

// Publish marks an existing landing page as published. Returns
// ErrLandingPageNotFound if no row matches.
func (r *Repository) Publish(ctx context.Context, hotelID uuid.UUID, locale string) (*LandingPage, error) {
	row := r.db.QueryRow(ctx, `
		UPDATE landing_pages
		SET status = 'published', published_at = NOW()
		WHERE hotel_id = $1 AND locale = $2
		RETURNING `+selectColumns,
		hotelID, locale,
	)
	lp, err := scanLandingPage(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrLandingPageNotFound
		}
		return nil, err
	}
	return lp, nil
}

// Unpublish flips status back to draft and clears published_at.
func (r *Repository) Unpublish(ctx context.Context, hotelID uuid.UUID, locale string) (*LandingPage, error) {
	row := r.db.QueryRow(ctx, `
		UPDATE landing_pages
		SET status = 'draft', published_at = NULL
		WHERE hotel_id = $1 AND locale = $2
		RETURNING `+selectColumns,
		hotelID, locale,
	)
	lp, err := scanLandingPage(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrLandingPageNotFound
		}
		return nil, err
	}
	return lp, nil
}

func (r *Repository) Delete(ctx context.Context, hotelID uuid.UUID, locale string) error {
	tag, err := r.db.Exec(ctx, `
		DELETE FROM landing_pages
		WHERE hotel_id = $1 AND locale = $2
	`, hotelID, locale)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrLandingPageNotFound
	}
	return nil
}

// GetPublishedBySlug serves the booking-web ISR endpoint. It joins hotels +
// landing_pages and only returns a row when both the hotel is live AND the
// landing page is published. Anything else => ErrLandingPageNotFound, so the
// public 404 cannot leak draft / suspended state. The returned response wraps
// the LandingPage with hotel context (timezone + currency) so the guest UI can
// format dates/prices without a second round-trip.
func (r *Repository) GetPublishedBySlug(ctx context.Context, slug, locale string) (*PublicLandingResponse, error) {
	var (
		lp                                          LandingPage
		brandingRaw, sectionsRaw, seoRaw, trackRaw []byte
		hotel                                       PublicHotelContext
	)
	err := r.db.QueryRow(ctx, `
		SELECT
			lp.id, lp.hotel_id, lp.locale, lp.status, lp.version,
			lp.branding, lp.sections, lp.seo, lp.tracking,
			lp.published_at, lp.created_at, lp.updated_at,
			h.name, h.slug, h.timezone, h.base_currency
		FROM landing_pages lp
		JOIN hotels h ON h.id = lp.hotel_id
		WHERE h.slug = $1
		  AND lp.locale = $2
		  AND h.status = 'live'
		  AND h.deleted_at IS NULL
		  AND lp.status = 'published'
	`, slug, locale).Scan(
		&lp.ID, &lp.HotelID, &lp.Locale, &lp.Status, &lp.Version,
		&brandingRaw, &sectionsRaw, &seoRaw, &trackRaw,
		&lp.PublishedAt, &lp.CreatedAt, &lp.UpdatedAt,
		&hotel.Name, &hotel.Slug, &hotel.Timezone, &hotel.Currency,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrLandingPageNotFound
		}
		return nil, err
	}
	if err := json.Unmarshal(brandingRaw, &lp.Branding); err != nil {
		return nil, fmt.Errorf("decode branding: %w", err)
	}
	lp.Sections = []Section{}
	if len(sectionsRaw) > 0 {
		if err := json.Unmarshal(sectionsRaw, &lp.Sections); err != nil {
			return nil, fmt.Errorf("decode sections: %w", err)
		}
	}
	if err := json.Unmarshal(seoRaw, &lp.SEO); err != nil {
		return nil, fmt.Errorf("decode seo: %w", err)
	}
	if err := json.Unmarshal(trackRaw, &lp.Tracking); err != nil {
		return nil, fmt.Errorf("decode tracking: %w", err)
	}
	return &PublicLandingResponse{LandingPage: lp, Hotel: hotel}, nil
}
