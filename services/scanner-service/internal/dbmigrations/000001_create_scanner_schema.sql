CREATE TABLE IF NOT EXISTS scanner_watchlist (
	repo TEXT PRIMARY KEY,
	last_seen_tag TEXT NOT NULL DEFAULT '',
	next_scan_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE scanner_watchlist
	DROP COLUMN IF EXISTS enabled;

CREATE TABLE IF NOT EXISTS scanner_releases (
	repo TEXT NOT NULL,
	tag TEXT NOT NULL,
	detected_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	PRIMARY KEY (repo, tag)
);

CREATE INDEX IF NOT EXISTS idx_scanner_watchlist_due
	ON scanner_watchlist (next_scan_at, repo);
