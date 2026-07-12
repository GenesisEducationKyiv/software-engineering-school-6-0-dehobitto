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

func (r *Repository) ApplyWatchCommand(ctx context.Context, payload contracts.RepoWatchCommandPayload, correlationID string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin watch command: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var operationErr error
	switch payload.Action {
	case contracts.RepoWatchActionStart:
		operationErr = startWatchingTx(ctx, tx, payload.Repo)
	case contracts.RepoWatchActionStop:
		operationErr = stopWatchingTx(ctx, tx, payload.Repo)
	default:
		operationErr = fmt.Errorf("unsupported repo watch action %q", payload.Action)
	}
	if operationErr != nil {
		if ackErr := insertWatchAckTx(ctx, tx, payload, correlationID, operationErr); ackErr != nil {
			return fmt.Errorf("%w; publish failure ack: %w", operationErr, ackErr)
		}
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit watch failure ack: %w", err)
		}
		return nil
	}

	if err := insertWatchAckTx(ctx, tx, payload, correlationID, nil); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit watch command: %w", err)
	}
	return nil
}

func startWatchingTx(ctx context.Context, tx pgx.Tx, repo string) error {
	_, err := tx.Exec(ctx, `
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

func stopWatchingTx(ctx context.Context, tx pgx.Tx, repo string) error {
	_, err := tx.Exec(ctx, `
DELETE FROM scanner_watchlist
WHERE repo = $1
`, repo)
	if err != nil {
		return fmt.Errorf("stop watching repo: %w", err)
	}
	return nil
}

func insertWatchAckTx(ctx context.Context, tx pgx.Tx, payload contracts.RepoWatchCommandPayload, correlationID string, cause error) error {
	eventID := uuid.NewString()
	event, raw, err := buildRepoWatchAckEvent(payload, eventID, correlationID, time.Now().UTC(), cause)
	if err != nil {
		return fmt.Errorf("marshal watch ack: %w", err)
	}
	return outbox.InsertTx(ctx, tx, outbox.Event{
		EventID:       eventID,
		EventType:     event.EventType,
		OccurredAt:    event.OccurredAt,
		Source:        event.Source,
		CorrelationID: correlationID,
		Topic:         contracts.TopicWatchlistSagaEvents,
		KafkaKey:      payload.Repo,
		Payload:       raw,
	})
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

func buildRepoWatchAckEvent(payload contracts.RepoWatchCommandPayload, eventID, correlationID string, occurredAt time.Time, cause error) (contracts.Envelope[contracts.RepoWatchAckPayload], []byte, error) {
	eventType := contracts.EventRepoWatchFailed
	errorMessage := ""
	if cause == nil {
		switch payload.Action {
		case contracts.RepoWatchActionStart:
			eventType = contracts.EventRepoWatchStarted
		case contracts.RepoWatchActionStop:
			eventType = contracts.EventRepoWatchStopped
		default:
			cause = fmt.Errorf("unsupported repo watch action %q", payload.Action)
			errorMessage = cause.Error()
		}
	} else {
		errorMessage = cause.Error()
	}

	event := contracts.Envelope[contracts.RepoWatchAckPayload]{
		EventID:       eventID,
		EventType:     eventType,
		OccurredAt:    occurredAt,
		Source:        "scanner-service",
		CorrelationID: correlationID,
		Payload: contracts.RepoWatchAckPayload{
			SagaID: payload.SagaID,
			Action: payload.Action,
			Repo:   payload.Repo,
			Error:  errorMessage,
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
