CREATE TABLE IF NOT EXISTS saga_instances (
	saga_id UUID PRIMARY KEY,
	saga_type TEXT NOT NULL,
	repo TEXT NOT NULL,
	action TEXT NOT NULL,
	status TEXT NOT NULL,
	attempts INTEGER NOT NULL DEFAULT 0,
	correlation_id UUID NOT NULL,
	next_retry_at TIMESTAMPTZ,
	deadline_at TIMESTAMPTZ NOT NULL,
	last_error TEXT,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_saga_instances_retry_due
	ON saga_instances (next_retry_at, status)
	WHERE status IN ('command_sent', 'retrying');
