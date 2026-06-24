ALTER TABLE notification_deliveries
	ADD COLUMN IF NOT EXISTS message TEXT NOT NULL DEFAULT '',
	ADD COLUMN IF NOT EXISTS next_attempt_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_notification_deliveries_next_attempt
	ON notification_deliveries (status, next_attempt_at)
	WHERE next_attempt_at IS NOT NULL;
