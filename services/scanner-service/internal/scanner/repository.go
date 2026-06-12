package scanner

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"subber/pkg/contracts"
	"subber/pkg/outbox"
)

type WatchedRepo struct {
	Repo        string
	LastSeenTag string
}

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	_, err := pool.Exec(ctx, `
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
`)
	if err != nil {
		return fmt.Errorf("migrate scanner schema: %w", err)
	}
	return nil
}

func (r *Repository) ClaimDue(ctx context.Context, limit int, nextScanIn time.Duration) ([]WatchedRepo, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin claim due repos: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	rows, err := tx.Query(ctx, `
SELECT repo, last_seen_tag
FROM scanner_watchlist
WHERE next_scan_at <= now()
ORDER BY next_scan_at, repo
FOR UPDATE SKIP LOCKED
LIMIT $1
`, limit)
	if err != nil {
		return nil, fmt.Errorf("select due repos: %w", err)
	}
	defer rows.Close()

	var repos []WatchedRepo
	for rows.Next() {
		var repo WatchedRepo
		if err := rows.Scan(&repo.Repo, &repo.LastSeenTag); err != nil {
			return nil, fmt.Errorf("scan due repo: %w", err)
		}
		repos = append(repos, repo)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate due repos: %w", err)
	}

	for _, repo := range repos {
		if _, err := tx.Exec(ctx, `
UPDATE scanner_watchlist
SET next_scan_at = now() + $2::interval
WHERE repo = $1
`, repo.Repo, durationInterval(nextScanIn)); err != nil {
			return nil, fmt.Errorf("reserve next scan for %s: %w", repo.Repo, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit due repo claim: %w", err)
	}
	return repos, nil
}

func (r *Repository) StartWatching(ctx context.Context, repo string) error {
	_, err := r.pool.Exec(ctx, `
INSERT INTO scanner_watchlist (repo, next_scan_at)
VALUES ($1, now())
ON CONFLICT (repo) DO UPDATE
SET next_scan_at = LEAST(scanner_watchlist.next_scan_at, now())
`, repo)
	if err != nil {
		return fmt.Errorf("start watching repo: %w", err)
	}
	return nil
}

func (r *Repository) StopWatching(ctx context.Context, repo string) error {
	_, err := r.pool.Exec(ctx, `
DELETE FROM scanner_watchlist
WHERE repo = $1
`, repo)
	if err != nil {
		return fmt.Errorf("stop watching repo: %w", err)
	}
	return nil
}

func (r *Repository) MarkReleaseDetected(ctx context.Context, repo, tag, correlationID string) (bool, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("begin release detection: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	inserted, err := insertRelease(ctx, tx, repo, tag)
	if err != nil {
		return false, err
	}
	if !inserted {
		return false, tx.Commit(ctx)
	}

	if _, err := tx.Exec(ctx, `
UPDATE scanner_watchlist
SET last_seen_tag = $2
WHERE repo = $1
`, repo, tag); err != nil {
		return false, fmt.Errorf("update last seen tag: %w", err)
	}

	eventID := uuid.NewString()
	event, payload, err := buildReleaseDetectedEvent(repo, tag, eventID, correlationID, time.Now().UTC())
	if err != nil {
		return false, fmt.Errorf("marshal release event: %w", err)
	}

	if err := outbox.InsertTx(ctx, tx, outbox.Event{
		EventID:       eventID,
		EventType:     event.EventType,
		OccurredAt:    event.OccurredAt,
		Source:        event.Source,
		CorrelationID: correlationID,
		Topic:         contracts.TopicReleaseEvents,
		KafkaKey:      repo,
		Payload:       payload,
	}); err != nil {
		return false, err
	}

	if err := tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("commit release detection: %w", err)
	}
	return true, nil
}

func buildReleaseDetectedEvent(repo, tag, eventID, correlationID string, occurredAt time.Time) (contracts.Envelope[contracts.ReleaseDetectedPayload], []byte, error) {
	event := contracts.Envelope[contracts.ReleaseDetectedPayload]{
		EventID:       eventID,
		EventType:     contracts.EventReleaseDetected,
		OccurredAt:    occurredAt,
		Source:        "scanner-service",
		CorrelationID: correlationID,
		Payload: contracts.ReleaseDetectedPayload{
			Repo: repo,
			Tag:  tag,
		},
	}
	raw, err := json.Marshal(event)
	return event, raw, err
}

func insertRelease(ctx context.Context, tx pgx.Tx, repo, tag string) (bool, error) {
	tagResult, err := tx.Exec(ctx, `
INSERT INTO scanner_releases (repo, tag)
VALUES ($1, $2)
ON CONFLICT (repo, tag) DO NOTHING
`, repo, tag)
	if err != nil {
		return false, fmt.Errorf("insert scanner release: %w", err)
	}
	return tagResult.RowsAffected() == 1, nil
}

func durationInterval(d time.Duration) string {
	return fmt.Sprintf("%f seconds", d.Seconds())
}
