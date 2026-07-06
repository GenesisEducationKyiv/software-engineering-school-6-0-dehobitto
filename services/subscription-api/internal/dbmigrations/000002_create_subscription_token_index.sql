CREATE UNIQUE INDEX IF NOT EXISTS idx_subscriptions_token
	ON subscriptions (token)
	WHERE token IS NOT NULL;
