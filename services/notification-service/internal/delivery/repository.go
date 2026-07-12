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
	Status         string
	AttemptCount   int
	LastError      string
	NextAttemptAt  *time.Time
	SentAt         *time.Time
}

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
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
	message,
	status
) VALUES ($1, $2, $3, $4, $5, $6, $7, 'pending')
ON CONFLICT (idempotency_key) DO UPDATE
SET updated_at = notification_deliveries.updated_at
RETURNING notification_id::text, idempotency_key, recipient_email, email_hash, repo, tag, message, status, attempt_count, last_error, next_attempt_at, sent_at
`, payload.NotificationID, payload.IdempotencyKey, payload.RecipientEmail, payload.EmailHash, payload.Repo, payload.Tag, payload.Message).Scan(
		&delivery.NotificationID,
		&delivery.IdempotencyKey,
		&delivery.RecipientEmail,
		&delivery.EmailHash,
		&delivery.Repo,
		&delivery.Tag,
		&delivery.Message,
		&delivery.Status,
		&delivery.AttemptCount,
		&delivery.LastError,
		&delivery.NextAttemptAt,
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
SET status = 'failed', attempt_count = attempt_count + 1, last_error = $2, next_attempt_at = $3, updated_at = now()
WHERE idempotency_key = $1
`, idempotencyKey, message, nextAttemptAt.UTC())
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
SET status = 'dead', attempt_count = attempt_count + 1, last_error = $2, next_attempt_at = NULL, updated_at = now()
WHERE idempotency_key = $1
`, idempotencyKey, message)
	if err != nil {
		return fmt.Errorf("mark notification dead: %w", err)
	}
	return nil
}

func (r *Repository) EnqueueDueRetries(ctx context.Context, limit int) (int, error) {
	if limit <= 0 {
		limit = 100
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("begin enqueue notification retries: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	rows, err := tx.Query(ctx, `
SELECT notification_id::text, idempotency_key, recipient_email, email_hash, repo, tag, message, status, attempt_count, last_error, next_attempt_at, sent_at
FROM notification_deliveries
WHERE status = 'failed'
	AND next_attempt_at IS NOT NULL
	AND next_attempt_at <= now()
ORDER BY next_attempt_at, updated_at, notification_id
FOR UPDATE SKIP LOCKED
LIMIT $1
`, limit)
	if err != nil {
		return 0, fmt.Errorf("fetch due notification retries: %w", err)
	}

	var deliveries []Delivery
	for rows.Next() {
		var delivery Delivery
		if err := rows.Scan(
			&delivery.NotificationID,
			&delivery.IdempotencyKey,
			&delivery.RecipientEmail,
			&delivery.EmailHash,
			&delivery.Repo,
			&delivery.Tag,
			&delivery.Message,
			&delivery.Status,
			&delivery.AttemptCount,
			&delivery.LastError,
			&delivery.NextAttemptAt,
			&delivery.SentAt,
		); err != nil {
			rows.Close()
			return 0, fmt.Errorf("scan due notification retry: %w", err)
		}
		deliveries = append(deliveries, delivery)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return 0, fmt.Errorf("iterate due notification retries: %w", err)
	}
	rows.Close()

	now := time.Now().UTC()
	for _, delivery := range deliveries {
		payload := contracts.NotificationSendRequestedPayload{
			NotificationID: delivery.NotificationID,
			IdempotencyKey: delivery.IdempotencyKey,
			RecipientEmail: delivery.RecipientEmail,
			EmailHash:      delivery.EmailHash,
			Repo:           delivery.Repo,
			Tag:            delivery.Tag,
			Message:        delivery.Message,
		}
		raw, err := marshalRetryEvent(payload, now, uuid.NewString())
		if err != nil {
			return 0, fmt.Errorf("marshal notification retry event: %w", err)
		}
		if err := outbox.InsertTx(ctx, tx, outbox.Event{
			EventID:       uuid.NewString(),
			EventType:     contracts.EventNotificationRequested,
			OccurredAt:    now,
			Source:        "notification-service",
			CorrelationID: delivery.NotificationID,
			Topic:         contracts.TopicNotificationCommands,
			KafkaKey:      delivery.EmailHash,
			Payload:       raw,
		}); err != nil {
			return 0, err
		}
		if _, err := tx.Exec(ctx, `
UPDATE notification_deliveries
SET status = 'pending', next_attempt_at = NULL, updated_at = now()
WHERE idempotency_key = $1
`, delivery.IdempotencyKey); err != nil {
			return 0, fmt.Errorf("mark notification retry pending: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit enqueue notification retries: %w", err)
	}
	return len(deliveries), nil
}

func marshalRetryEvent(payload contracts.NotificationSendRequestedPayload, occurredAt time.Time, eventID string) ([]byte, error) {
	event := contracts.Envelope[contracts.NotificationSendRequestedPayload]{
		EventID:       eventID,
		EventType:     contracts.EventNotificationRequested,
		OccurredAt:    occurredAt,
		Source:        "notification-service",
		CorrelationID: payload.NotificationID,
		Payload:       payload,
	}
	return json.Marshal(event)
}
