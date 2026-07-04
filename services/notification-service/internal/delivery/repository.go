package delivery

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"subber/pkg/contracts"
	"subber/pkg/outbox"
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
	Message        string
	CorrelationID  string
	Status         string
	AttemptCount   int
	LastError      string
	SentAt         *time.Time
	NextAttemptAt  *time.Time
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
	message TEXT NOT NULL DEFAULT '',
	correlation_id UUID NOT NULL,
	status TEXT NOT NULL DEFAULT 'pending',
	attempt_count INT NOT NULL DEFAULT 0,
	last_error TEXT NOT NULL DEFAULT '',
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	next_attempt_at TIMESTAMPTZ,
	sent_at TIMESTAMPTZ
);

ALTER TABLE notification_deliveries
	ADD COLUMN IF NOT EXISTS message TEXT NOT NULL DEFAULT '';

ALTER TABLE notification_deliveries
	ADD COLUMN IF NOT EXISTS next_attempt_at TIMESTAMPTZ;

ALTER TABLE notification_deliveries
	ADD COLUMN IF NOT EXISTS correlation_id UUID;

UPDATE notification_deliveries
SET correlation_id = notification_id
WHERE correlation_id IS NULL;

ALTER TABLE notification_deliveries
	ALTER COLUMN correlation_id SET NOT NULL;

CREATE INDEX IF NOT EXISTS idx_notification_deliveries_status
	ON notification_deliveries (status, updated_at);

CREATE INDEX IF NOT EXISTS idx_notification_deliveries_retry_due
	ON notification_deliveries (status, next_attempt_at)
	WHERE next_attempt_at IS NOT NULL;
`)
	if err != nil {
		return fmt.Errorf("migrate notification schema: %w", err)
	}
	return nil
}

func (r *Repository) UpsertPending(ctx context.Context, payload contracts.NotificationSendRequestedPayload, correlationID string) (Delivery, error) {
	var delivery Delivery
	err := r.pool.QueryRow(ctx, `
INSERT INTO notification_deliveries (
	notification_id,
	idempotency_key,
	recipient_email,
	email_hash,
	repo,
	tag,
	message,
	correlation_id,
	status
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 'pending')
ON CONFLICT (idempotency_key) DO UPDATE
SET updated_at = notification_deliveries.updated_at
RETURNING notification_id::text, idempotency_key, recipient_email, email_hash, repo, tag, message, correlation_id::text, status, attempt_count, last_error, sent_at, next_attempt_at
`, payload.NotificationID, payload.IdempotencyKey, payload.RecipientEmail, payload.EmailHash, payload.Repo, payload.Tag, payload.Message, correlationID).Scan(
		&delivery.NotificationID,
		&delivery.IdempotencyKey,
		&delivery.RecipientEmail,
		&delivery.EmailHash,
		&delivery.Repo,
		&delivery.Tag,
		&delivery.Message,
		&delivery.CorrelationID,
		&delivery.Status,
		&delivery.AttemptCount,
		&delivery.LastError,
		&delivery.SentAt,
		&delivery.NextAttemptAt,
	)
	if err != nil {
		return Delivery{}, fmt.Errorf("upsert notification delivery: %w", err)
	}
	return delivery, nil
}

func (r *Repository) MarkSent(ctx context.Context, idempotencyKey string) error {
	_, err := r.pool.Exec(ctx, `
UPDATE notification_deliveries
SET status = 'sent', sent_at = now(), updated_at = now(), last_error = '', next_attempt_at = NULL
WHERE idempotency_key = $1
`, idempotencyKey)
	if err != nil {
		return fmt.Errorf("mark notification sent: %w", err)
	}
	return nil
}

func (r *Repository) MarkFailed(ctx context.Context, idempotencyKey string, cause error, nextAttemptAt time.Time) error {
	message := ""
	if cause != nil {
		message = cause.Error()
	}
	_, err := r.pool.Exec(ctx, `
UPDATE notification_deliveries
SET status = 'failed', attempt_count = attempt_count + 1, last_error = $2, updated_at = now(), next_attempt_at = $3
WHERE idempotency_key = $1
`, idempotencyKey, message, nextAttemptAt)
	if err != nil {
		return fmt.Errorf("mark notification failed: %w", err)
	}
	return nil
}

func (r *Repository) MarkDead(ctx context.Context, payload contracts.NotificationSendRequestedPayload, correlationID string, cause error) error {
	message := ""
	if cause != nil {
		message = cause.Error()
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin dead-letter notification: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	_, err = tx.Exec(ctx, `
UPDATE notification_deliveries
SET status = 'dead', attempt_count = attempt_count + 1, last_error = $2, updated_at = now(), next_attempt_at = NULL
WHERE idempotency_key = $1
`, payload.IdempotencyKey, message)
	if err != nil {
		return fmt.Errorf("mark notification dead: %w", err)
	}

	dlqEvent, err := buildDLQOutboxEvent(payload, correlationID, time.Now().UTC())
	if err != nil {
		return err
	}
	if err := outbox.InsertTx(ctx, tx, dlqEvent); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

type ScheduledRetry struct {
	Payload       contracts.NotificationSendRequestedPayload
	CorrelationID string
}

func (r *Repository) FetchDueRetries(ctx context.Context, limit int) ([]ScheduledRetry, error) {
	if limit <= 0 {
		limit = 100
	}

	rows, err := r.pool.Query(ctx, `
SELECT notification_id::text, idempotency_key, recipient_email, email_hash, repo, tag, message, correlation_id::text
FROM notification_deliveries
WHERE status = 'failed' AND next_attempt_at IS NOT NULL AND next_attempt_at <= now()
ORDER BY next_attempt_at, updated_at
LIMIT $1
`, limit)
	if err != nil {
		return nil, fmt.Errorf("fetch due notification retries: %w", err)
	}
	defer rows.Close()

	var retries []ScheduledRetry
	for rows.Next() {
		var retry ScheduledRetry
		if err := rows.Scan(
			&retry.Payload.NotificationID,
			&retry.Payload.IdempotencyKey,
			&retry.Payload.RecipientEmail,
			&retry.Payload.EmailHash,
			&retry.Payload.Repo,
			&retry.Payload.Tag,
			&retry.Payload.Message,
			&retry.CorrelationID,
		); err != nil {
			return nil, fmt.Errorf("scan due notification retry: %w", err)
		}
		retries = append(retries, retry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate due notification retries: %w", err)
	}
	return retries, nil
}

func buildDLQOutboxEvent(payload contracts.NotificationSendRequestedPayload, correlationID string, occurredAt time.Time) (outbox.Event, error) {
	eventID := uuid.NewString()
	event := contracts.Envelope[contracts.NotificationSendRequestedPayload]{
		EventID:       eventID,
		EventType:     contracts.EventNotificationRequested,
		OccurredAt:    occurredAt,
		Source:        "notification-service",
		CorrelationID: correlationID,
		Payload:       payload,
	}
	raw, err := json.Marshal(event)
	if err != nil {
		return outbox.Event{}, fmt.Errorf("marshal notification dlq: %w", err)
	}
	return outbox.Event{
		EventID:       eventID,
		EventType:     contracts.EventNotificationRequested,
		OccurredAt:    occurredAt,
		Source:        "notification-service",
		CorrelationID: correlationID,
		Topic:         contracts.TopicNotificationDLQ,
		KafkaKey:      payload.EmailHash,
		Payload:       raw,
	}, nil
}
