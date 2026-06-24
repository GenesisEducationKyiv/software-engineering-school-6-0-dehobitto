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
