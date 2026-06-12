package delivery

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"subber/pkg/contracts"
)

const (
	StatusPending = "pending"
	StatusSent    = "sent"
	StatusFailed  = "failed"
	StatusDead    = "dead"
)

type Delivery struct {
	NotificationID string
	IdempotencyKey string
	RecipientEmail string
	EmailHash      string
	Repo           string
	Tag            string
	Status         string
	AttemptCount   int
	LastError      string
	SentAt         *time.Time
}

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	_, err := pool.Exec(ctx, `
CREATE TABLE IF NOT EXISTS notification_deliveries (
	notification_id UUID PRIMARY KEY,
	idempotency_key TEXT UNIQUE NOT NULL,
	recipient_email TEXT NOT NULL,
	email_hash TEXT NOT NULL,
	repo TEXT NOT NULL,
	tag TEXT NOT NULL,
	status TEXT NOT NULL DEFAULT 'pending',
	attempt_count INT NOT NULL DEFAULT 0,
	last_error TEXT NOT NULL DEFAULT '',
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	sent_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_notification_deliveries_status
	ON notification_deliveries (status, updated_at);
`)
	if err != nil {
		return fmt.Errorf("migrate notification schema: %w", err)
	}
	return nil
}

func (r *Repository) UpsertPending(ctx context.Context, payload contracts.NotificationSendRequestedPayload) (Delivery, error) {
	var delivery Delivery
	err := r.pool.QueryRow(ctx, `
INSERT INTO notification_deliveries (
	notification_id,
	idempotency_key,
	recipient_email,
	email_hash,
	repo,
	tag,
	status
) VALUES ($1, $2, $3, $4, $5, $6, 'pending')
ON CONFLICT (idempotency_key) DO UPDATE
SET updated_at = notification_deliveries.updated_at
RETURNING notification_id::text, idempotency_key, recipient_email, email_hash, repo, tag, status, attempt_count, last_error, sent_at
`, payload.NotificationID, payload.IdempotencyKey, payload.RecipientEmail, payload.EmailHash, payload.Repo, payload.Tag).Scan(
		&delivery.NotificationID,
		&delivery.IdempotencyKey,
		&delivery.RecipientEmail,
		&delivery.EmailHash,
		&delivery.Repo,
		&delivery.Tag,
		&delivery.Status,
		&delivery.AttemptCount,
		&delivery.LastError,
		&delivery.SentAt,
	)
	if err != nil {
		return Delivery{}, fmt.Errorf("upsert notification delivery: %w", err)
	}
	return delivery, nil
}

func (r *Repository) MarkSent(ctx context.Context, idempotencyKey string) error {
	_, err := r.pool.Exec(ctx, `
UPDATE notification_deliveries
SET status = 'sent', sent_at = now(), updated_at = now(), last_error = ''
WHERE idempotency_key = $1
`, idempotencyKey)
	if err != nil {
		return fmt.Errorf("mark notification sent: %w", err)
	}
	return nil
}

func (r *Repository) MarkFailed(ctx context.Context, idempotencyKey string, cause error) error {
	message := ""
	if cause != nil {
		message = cause.Error()
	}
	_, err := r.pool.Exec(ctx, `
UPDATE notification_deliveries
SET status = 'failed', attempt_count = attempt_count + 1, last_error = $2, updated_at = now()
WHERE idempotency_key = $1
`, idempotencyKey, message)
	if err != nil {
		return fmt.Errorf("mark notification failed: %w", err)
	}
	return nil
}

func (r *Repository) MarkDead(ctx context.Context, idempotencyKey string, cause error) error {
	message := ""
	if cause != nil {
		message = cause.Error()
	}
	_, err := r.pool.Exec(ctx, `
UPDATE notification_deliveries
SET status = 'dead', attempt_count = attempt_count + 1, last_error = $2, updated_at = now()
WHERE idempotency_key = $1
`, idempotencyKey, message)
	if err != nil {
		return fmt.Errorf("mark notification dead: %w", err)
	}
	return nil
}
