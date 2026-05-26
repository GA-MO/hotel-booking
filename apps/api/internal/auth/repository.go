package auth

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"time"

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

// CreateAccountWithOwner creates an account and its first user in a single TX.
// Returns the created user (without password_hash field populated for safety).
func (r *Repository) CreateAccountWithOwner(
	ctx context.Context,
	email, passwordHash, name, locale, country string,
) (*User, error) {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // rollback errors are noise after commit success

	var accountID uuid.UUID
	if err := tx.QueryRow(ctx, `
		INSERT INTO accounts (billing_email, country)
		VALUES ($1, NULLIF($2, ''))
		RETURNING id
	`, email, country).Scan(&accountID); err != nil {
		return nil, fmt.Errorf("insert account: %w", err)
	}

	var u User
	err = tx.QueryRow(ctx, `
		INSERT INTO users (account_id, email, password_hash, name, role, locale)
		VALUES ($1, $2, $3, $4, 'owner', COALESCE(NULLIF($5, ''), 'en'))
		RETURNING id, account_id, email, name, role, locale, email_verified_at, created_at
	`, accountID, email, passwordHash, name, locale).Scan(
		&u.ID, &u.AccountID, &u.Email, &u.Name, &u.Role, &u.Locale, &u.EmailVerifiedAt, &u.CreatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrEmailAlreadyExists
		}
		return nil, fmt.Errorf("insert user: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}
	return &u, nil
}

// GetUserByEmail fetches a user along with their password hash for credential check.
func (r *Repository) GetUserByEmail(ctx context.Context, email string) (*User, string, error) {
	var u User
	var hash string
	err := r.db.QueryRow(ctx, `
		SELECT id, account_id, email, name, role, locale, email_verified_at, created_at, password_hash
		FROM users
		WHERE email = $1 AND deleted_at IS NULL
	`, email).Scan(
		&u.ID, &u.AccountID, &u.Email, &u.Name, &u.Role, &u.Locale, &u.EmailVerifiedAt, &u.CreatedAt, &hash,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, "", ErrUserNotFound
		}
		return nil, "", err
	}
	return &u, hash, nil
}

func (r *Repository) GetUserByID(ctx context.Context, id uuid.UUID) (*User, error) {
	var u User
	err := r.db.QueryRow(ctx, `
		SELECT id, account_id, email, name, role, locale, email_verified_at, created_at
		FROM users
		WHERE id = $1 AND deleted_at IS NULL
	`, id).Scan(
		&u.ID, &u.AccountID, &u.Email, &u.Name, &u.Role, &u.Locale, &u.EmailVerifiedAt, &u.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return &u, nil
}

func (r *Repository) UpdateLastLogin(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.Exec(ctx, `UPDATE users SET last_login_at = NOW() WHERE id = $1`, id)
	return err
}

// CreateSession inserts a refresh-token session record.
func (r *Repository) CreateSession(
	ctx context.Context,
	userID uuid.UUID,
	refreshTokenHash, userAgent string,
	ip netip.Addr,
	expiresAt time.Time,
) (*Session, error) {
	var s Session
	var ipArg any
	if ip.IsValid() {
		ipArg = ip.String()
	}
	err := r.db.QueryRow(ctx, `
		INSERT INTO sessions (user_id, refresh_token_hash, user_agent, ip_address, expires_at)
		VALUES ($1, $2, NULLIF($3, ''), $4, $5)
		RETURNING id, user_id, expires_at, revoked_at, rotated_to, created_at, last_used_at
	`, userID, refreshTokenHash, userAgent, ipArg, expiresAt).Scan(
		&s.ID, &s.UserID, &s.ExpiresAt, &s.RevokedAt, &s.RotatedTo, &s.CreatedAt, &s.LastUsedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert session: %w", err)
	}
	return &s, nil
}

// GetSessionByTokenHash looks up a session by its stored refresh token hash.
func (r *Repository) GetSessionByTokenHash(ctx context.Context, hash string) (*Session, error) {
	var s Session
	err := r.db.QueryRow(ctx, `
		SELECT id, user_id, expires_at, revoked_at, rotated_to, created_at, last_used_at
		FROM sessions
		WHERE refresh_token_hash = $1
	`, hash).Scan(
		&s.ID, &s.UserID, &s.ExpiresAt, &s.RevokedAt, &s.RotatedTo, &s.CreatedAt, &s.LastUsedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrSessionNotFound
		}
		return nil, err
	}
	return &s, nil
}

// RevokeSession marks a session as revoked. Optionally records the new session it rotated to.
func (r *Repository) RevokeSession(ctx context.Context, id uuid.UUID, rotatedTo *uuid.UUID) error {
	_, err := r.db.Exec(ctx, `
		UPDATE sessions
		SET revoked_at = NOW(), rotated_to = $2
		WHERE id = $1 AND revoked_at IS NULL
	`, id, rotatedTo)
	return err
}

// RevokeAllUserSessions revokes every active session for a user (used on token-reuse detection).
func (r *Repository) RevokeAllUserSessions(ctx context.Context, userID uuid.UUID) error {
	_, err := r.db.Exec(ctx, `
		UPDATE sessions SET revoked_at = NOW()
		WHERE user_id = $1 AND revoked_at IS NULL
	`, userID)
	return err
}

func isUniqueViolation(err error) bool {
	const uniqueViolation = "23505"
	type pgErr interface{ SQLState() string }
	var pe pgErr
	if errors.As(err, &pe) {
		return pe.SQLState() == uniqueViolation
	}
	return false
}
