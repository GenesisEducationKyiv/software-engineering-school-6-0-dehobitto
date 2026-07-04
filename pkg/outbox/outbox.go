package outbox

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Event struct {
	EventID         string
	EventType       string
	OccurredAt      time.Time
	Source          string
	CorrelationID   string
	Topic           string
	KafkaKey        string
	Payload         []byte
	PublishedAt     *time.Time
	LockedUntil     *time.Time
	PublishAttempts int
	LastError       *string
}

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	_, err := pool.Exec(ctx, `
CREATE TABLE IF NOT EXISTS outbox_events (
	event_id UUID PRIMARY KEY,
	event_type TEXT NOT NULL,
	occurred_at TIMESTAMPTZ NOT NULL,
	source TEXT NOT NULL,
	correlation_id UUID NOT NULL,
	topic TEXT NOT NULL,
	kafka_key TEXT NOT NULL,
	payload JSONB NOT NULL,
	published_at TIMESTAMPTZ,
	locked_until TIMESTAMPTZ,
	publish_attempts INT NOT NULL DEFAULT 0,
	last_error TEXT
);

ALTER TABLE outbox_events
	ADD COLUMN IF NOT EXISTS locked_until TIMESTAMPTZ;

DROP INDEX IF EXISTS idx_outbox_events_unpublished;

CREATE INDEX idx_outbox_events_unpublished
	ON outbox_events (occurred_at, event_id)
	WHERE published_at IS NULL AND locked_until IS NULL;
`)
	if err != nil {
		return fmt.Errorf("migrate outbox: %w", err)
	}
	return nil
}

func InsertTx(ctx context.Context, tx pgx.Tx, e Event) error {
	if e.OccurredAt.IsZero() {
		e.OccurredAt = time.Now().UTC()
	}
	if !json.Valid(e.Payload) {
		return fmt.Errorf("outbox payload must be valid JSON")
	}

	_, err := tx.Exec(ctx, `
INSERT INTO outbox_events (
	event_id,
	event_type,
	occurred_at,
	source,
	correlation_id,
	topic,
	kafka_key,
	payload
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (event_id) DO NOTHING
`, e.EventID, e.EventType, e.OccurredAt, e.Source, e.CorrelationID, e.Topic, e.KafkaKey, e.Payload)
	if err != nil {
		return fmt.Errorf("insert outbox event: %w", err)
	}
	return nil
}

func (r *Repository) FetchUnpublished(ctx context.Context, limit int) ([]Event, error) {
	rows, err := r.pool.Query(ctx, `
WITH candidates AS (
	SELECT event_id
	FROM outbox_events
	WHERE published_at IS NULL
		AND (locked_until IS NULL OR locked_until < now())
	ORDER BY occurred_at, event_id
	FOR UPDATE SKIP LOCKED
	LIMIT $1
)
UPDATE outbox_events AS e
SET locked_until = now() + interval '30 seconds'
FROM candidates AS c
WHERE e.event_id = c.event_id
RETURNING e.event_id::text, e.event_type, e.occurred_at, e.source, e.correlation_id::text, e.topic, e.kafka_key, e.payload, e.published_at, e.locked_until, e.publish_attempts, e.last_error
`, limit)
	if err != nil {
		return nil, fmt.Errorf("fetch unpublished outbox events: %w", err)
	}
	defer rows.Close()

	var events []Event
	for rows.Next() {
		var e Event
		if err := rows.Scan(
			&e.EventID,
			&e.EventType,
			&e.OccurredAt,
			&e.Source,
			&e.CorrelationID,
			&e.Topic,
			&e.KafkaKey,
			&e.Payload,
			&e.PublishedAt,
			&e.LockedUntil,
			&e.PublishAttempts,
			&e.LastError,
		); err != nil {
			return nil, fmt.Errorf("scan outbox event: %w", err)
		}
		events = append(events, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate outbox events: %w", err)
	}
	return events, nil
}

func (r *Repository) MarkPublished(ctx context.Context, eventID string) error {
	_, err := r.pool.Exec(ctx, `
UPDATE outbox_events
SET published_at = now(), locked_until = NULL, last_error = NULL
WHERE event_id = $1
`, eventID)
	if err != nil {
		return fmt.Errorf("mark outbox event published: %w", err)
	}
	return nil
}

func (r *Repository) MarkFailed(ctx context.Context, eventID string, cause error) error {
	message := ""
	if cause != nil {
		message = cause.Error()
	}
	_, err := r.pool.Exec(ctx, `
UPDATE outbox_events
SET publish_attempts = publish_attempts + 1, locked_until = NULL, last_error = $2
WHERE event_id = $1
`, eventID, message)
	if err != nil {
		return fmt.Errorf("mark outbox event failed: %w", err)
	}
	return nil
}
