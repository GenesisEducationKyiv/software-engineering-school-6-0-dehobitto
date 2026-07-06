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
