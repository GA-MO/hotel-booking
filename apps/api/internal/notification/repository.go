package notification

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
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

const selectColumns = `
	id, channel, template, recipient, payload,
	COALESCE(related_type, ''), related_id,
	status, attempts, COALESCE(last_error, ''),
	send_after, sent_at, created_at, updated_at
`

// Enqueue inserts a notification row in the 'queued' status. send_after may
// be zero — the DEFAULT NOW() kicks in. Caller is expected to have already
// validated the channel/template/recipient at the service layer.
func (r *Repository) Enqueue(ctx context.Context, n Notification) (*Notification, error) {
	payload := n.Payload
	if payload == nil {
		payload = map[string]any{}
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encode payload: %w", err)
	}

	var sendAfter any
	if !n.SendAfter.IsZero() {
		sendAfter = n.SendAfter
	}

	row := r.db.QueryRow(ctx, `
		INSERT INTO notifications (
			channel, template, recipient, payload,
			related_type, related_id, send_after
		)
		VALUES (
			$1, $2, $3, $4::jsonb,
			NULLIF($5, ''), $6,
			COALESCE($7::timestamptz, NOW())
		)
		RETURNING `+selectColumns,
		string(n.Channel), string(n.Template), n.Recipient, payloadJSON,
		n.RelatedType, n.RelatedID, sendAfter,
	)
	return scanNotification(row)
}

// ClaimBatch atomically transitions up to `limit` queued rows whose
// send_after <= now to 'sending', returning them for dispatch. The CTE +
// FOR UPDATE SKIP LOCKED ensures that two worker processes hitting the
// queue concurrently won't grab the same row.
func (r *Repository) ClaimBatch(ctx context.Context, limit int, now time.Time) ([]Notification, error) {
	if limit <= 0 {
		limit = 50
	}
	// Note: alias the CTE's id as `claimed_id` so the RETURNING clause's
	// unqualified column references (id, channel, ...) resolve unambiguously
	// to the UPDATE target row. Without the rename Postgres errors with
	// "column reference 'id' is ambiguous" (42702).
	rows, err := r.db.Query(ctx, `
		WITH claimed AS (
			SELECT id AS claimed_id FROM notifications
			WHERE status = 'queued' AND send_after <= $2
			ORDER BY send_after
			FOR UPDATE SKIP LOCKED
			LIMIT $1
		)
		UPDATE notifications
		SET status = 'sending', attempts = attempts + 1
		FROM claimed
		WHERE notifications.id = claimed.claimed_id
		RETURNING `+selectColumns,
		limit, now,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []Notification{}
	for rows.Next() {
		n, err := scanNotification(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *n)
	}
	return out, rows.Err()
}

// MarkSent flips status to 'sent' and records sent_at = NOW().
func (r *Repository) MarkSent(ctx context.Context, id uuid.UUID) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE notifications
		SET status = 'sent', sent_at = NOW(), last_error = NULL
		WHERE id = $1 AND status = 'sending'
	`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotificationNotFound
	}
	return nil
}

// MarkFailed bumps last_error. If the row's attempts has crossed MaxAttempts
// (or the caller forced deadLetter), it lands in 'dead'. Otherwise it's
// pushed back to 'queued' with exponential backoff: send_after = NOW() +
// 2^attempts minutes (so 2,4,8,16 minutes for attempts 1..4).
func (r *Repository) MarkFailed(ctx context.Context, id uuid.UUID, sendErr string, deadLetter bool) error {
	var attempts int
	if err := r.db.QueryRow(ctx,
		`SELECT attempts FROM notifications WHERE id = $1`, id,
	).Scan(&attempts); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotificationNotFound
		}
		return err
	}

	if deadLetter || attempts >= MaxAttempts {
		_, err := r.db.Exec(ctx, `
			UPDATE notifications
			SET status = 'dead', last_error = $2
			WHERE id = $1
		`, id, sendErr)
		return err
	}

	backoff := backoffFor(attempts)
	_, err := r.db.Exec(ctx, `
		UPDATE notifications
		SET status = 'queued', last_error = $2, send_after = NOW() + $3::interval
		WHERE id = $1
	`, id, sendErr, fmt.Sprintf("%d seconds", int(backoff.Seconds())))
	return err
}

// backoffFor returns the delay before the next attempt. attempts is the
// number we just *failed* on (already incremented by ClaimBatch). We use
// minutes-based exponential to keep test-friendly numbers while still
// being polite to providers: 2, 4, 8, 16 min.
func backoffFor(attempts int) time.Duration {
	if attempts < 1 {
		attempts = 1
	}
	// Cap exponent so we never overflow / wait > 32 minutes.
	exp := attempts
	if exp > 5 {
		exp = 5
	}
	mins := math.Pow(2, float64(exp))
	return time.Duration(mins) * time.Minute
}

// GetByID fetches one row for the admin debug endpoint.
func (r *Repository) GetByID(ctx context.Context, id uuid.UUID) (*Notification, error) {
	row := r.db.QueryRow(ctx, `SELECT `+selectColumns+` FROM notifications WHERE id = $1`, id)
	n, err := scanNotification(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotificationNotFound
		}
		return nil, err
	}
	return n, nil
}

// ListLatest returns the most recently created rows (any status), for the
// admin debug list endpoint.
func (r *Repository) ListLatest(ctx context.Context, limit int) ([]Notification, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := r.db.Query(ctx, `
		SELECT `+selectColumns+`
		FROM notifications
		ORDER BY created_at DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []Notification{}
	for rows.Next() {
		n, err := scanNotification(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *n)
	}
	return out, rows.Err()
}

func scanNotification(row pgx.Row) (*Notification, error) {
	var (
		n                       Notification
		channel, template, st   string
		payloadRaw              []byte
	)
	if err := row.Scan(
		&n.ID, &channel, &template, &n.Recipient, &payloadRaw,
		&n.RelatedType, &n.RelatedID,
		&st, &n.Attempts, &n.LastError,
		&n.SendAfter, &n.SentAt, &n.CreatedAt, &n.UpdatedAt,
	); err != nil {
		return nil, err
	}
	n.Channel = Channel(channel)
	n.Template = Template(template)
	n.Status = Status(st)
	n.Payload = map[string]any{}
	if len(payloadRaw) > 0 {
		if err := json.Unmarshal(payloadRaw, &n.Payload); err != nil {
			return nil, fmt.Errorf("decode payload: %w", err)
		}
		if n.Payload == nil {
			n.Payload = map[string]any{}
		}
	}
	return &n, nil
}
