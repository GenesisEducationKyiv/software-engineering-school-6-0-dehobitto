package subscription

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"subber/pkg/contracts"
	"subber/pkg/outbox"
	"subber/pkg/requestid"
)

type Repository struct {
	pool *pgxpool.Pool
}

type ConfirmSubscriptionResult struct {
	Repo              string
	Email             string
	WasFirstConfirmed bool
}

type subscriptionExecer interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) SubscriptionExists(ctx context.Context, email, repo string) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM subscriptions WHERE email = $1 AND repo = $2)`, email, repo).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check subscription exists: %w", err)
	}
	return exists, nil
}

func (r *Repository) SaveSubscription(ctx context.Context, sub Subscription) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin save subscription: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	if err := saveSubscription(ctx, tx, sub); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *Repository) DeleteUnconfirmedSubscription(ctx context.Context, email, repo, token string) error {
	tag, err := r.pool.Exec(ctx, `
DELETE FROM subscriptions
WHERE email = $1
	AND repo = $2
	AND token = $3
	AND confirmed = false
`, email, repo, token)
	if err != nil {
		return fmt.Errorf("delete unconfirmed subscription compensation: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("unconfirmed subscription compensation skipped")
	}
	return nil
}

func saveSubscription(ctx context.Context, execer subscriptionExecer, sub Subscription) error {
	_, err := execer.Exec(ctx, `
INSERT INTO subscriptions (email, repo, confirmed, last_seen_tag, token)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (email, repo) DO UPDATE
SET last_seen_tag = EXCLUDED.last_seen_tag
`, sub.Email, sub.Repo, sub.Confirmed, sub.LastSeenTag, sub.Token)
	if err != nil {
		return fmt.Errorf("save subscription: %w", err)
	}
	return nil
}

func (r *Repository) GetSubscriptions(ctx context.Context, email string) ([]Subscription, error) {
	rows, err := r.pool.Query(ctx, `
SELECT email, repo, confirmed, last_seen_tag
FROM subscriptions
WHERE email = $1 AND confirmed = true
`, email)
	if err != nil {
		return nil, fmt.Errorf("query subscriptions: %w", err)
	}
	defer rows.Close()

	var subscriptions []Subscription
	for rows.Next() {
		var sub Subscription
		if err := rows.Scan(&sub.Email, &sub.Repo, &sub.Confirmed, &sub.LastSeenTag); err != nil {
			return nil, fmt.Errorf("scan subscription: %w", err)
		}
		subscriptions = append(subscriptions, sub)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate subscriptions: %w", err)
	}
	return subscriptions, nil
}

func (r *Repository) GetSubscribers(ctx context.Context, repo string) ([]string, error) {
	rows, err := r.pool.Query(ctx, `
SELECT DISTINCT email
FROM subscriptions
WHERE repo = $1 AND confirmed = true
`, repo)
	if err != nil {
		return nil, fmt.Errorf("query subscribers: %w", err)
	}
	defer rows.Close()

	var subscribers []string
	for rows.Next() {
		var email string
		if err := rows.Scan(&email); err != nil {
			return nil, fmt.Errorf("scan subscriber: %w", err)
		}
		subscribers = append(subscribers, email)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate subscribers: %w", err)
	}
	return subscribers, nil
}

func (r *Repository) ConfirmSubscriptionByToken(ctx context.Context, token string) (ConfirmSubscriptionResult, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return ConfirmSubscriptionResult{}, fmt.Errorf("begin confirm subscription: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var repo string
	var email string
	var wasConfirmed bool
	if err := tx.QueryRow(ctx, `SELECT repo, email, confirmed FROM subscriptions WHERE token = $1 FOR UPDATE`, token).Scan(&repo, &email, &wasConfirmed); err != nil {
		return ConfirmSubscriptionResult{}, ErrTokenNotFound
	}

	var confirmedBefore int
	if err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM subscriptions WHERE repo = $1 AND confirmed = true`, repo).Scan(&confirmedBefore); err != nil {
		return ConfirmSubscriptionResult{}, fmt.Errorf("count confirmed subscriptions: %w", err)
	}

	if _, err := tx.Exec(ctx, `UPDATE subscriptions SET confirmed = true WHERE token = $1`, token); err != nil {
		return ConfirmSubscriptionResult{}, fmt.Errorf("confirm subscription: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return ConfirmSubscriptionResult{}, err
	}
	return ConfirmSubscriptionResult{
		Repo:              repo,
		Email:             email,
		WasFirstConfirmed: !wasConfirmed && confirmedBefore == 0,
	}, nil
}

func (r *Repository) RequestRepoWatchSaga(ctx context.Context, action, repo, email string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin repo watch saga request: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	if err := insertRepoWatchSagaRequest(ctx, tx, action, repo, email); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *Repository) Unsubscribe(ctx context.Context, token string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin unsubscribe: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var repo string
	var wasConfirmed bool
	if err := tx.QueryRow(ctx, `SELECT repo, confirmed FROM subscriptions WHERE token = $1 FOR UPDATE`, token).Scan(&repo, &wasConfirmed); err != nil {
		return fmt.Errorf("token not found")
	}

	if _, err := tx.Exec(ctx, `DELETE FROM subscriptions WHERE token = $1`, token); err != nil {
		return fmt.Errorf("delete subscription: %w", err)
	}

	if wasConfirmed {
		var confirmedAfter int
		if err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM subscriptions WHERE repo = $1 AND confirmed = true`, repo).Scan(&confirmedAfter); err != nil {
			return fmt.Errorf("count remaining confirmed subscriptions: %w", err)
		}
		if confirmedAfter == 0 {
			if err := insertRepoWatchSagaRequest(ctx, tx, contracts.RepoWatchActionStop, repo, ""); err != nil {
				return err
			}
		}
	}
	return tx.Commit(ctx)
}

func insertRepoWatchSagaRequest(ctx context.Context, tx pgx.Tx, action, repo, email string) error {
	sagaID := uuid.NewString()
	correlationID := correlationIDFromContext(ctx, sagaID)
	occurredAt := time.Now().UTC()
	event, payload, err := buildRepoWatchSagaRequest(action, repo, email, sagaID, correlationID, occurredAt)
	if err != nil {
		return fmt.Errorf("marshal repo watch saga request: %w", err)
	}
	return outbox.InsertTx(ctx, tx, outbox.Event{
		EventID:       sagaID,
		EventType:     event.EventType,
		OccurredAt:    event.OccurredAt,
		Source:        event.Source,
		CorrelationID: correlationID,
		Topic:         contracts.TopicWatchlistSagaRequests,
		KafkaKey:      repo,
		Payload:       payload,
	})
}

func buildRepoWatchSagaRequest(action, repo, email, sagaID, correlationID string, occurredAt time.Time) (contracts.Envelope[contracts.RepoWatchSagaPayload], []byte, error) {
	event := contracts.Envelope[contracts.RepoWatchSagaPayload]{
		EventID:       sagaID,
		EventType:     contracts.EventRepoWatchSagaRequested,
		OccurredAt:    occurredAt,
		Source:        "subscription-api",
		CorrelationID: correlationID,
		Payload: contracts.RepoWatchSagaPayload{
			SagaID: sagaID,
			Action: action,
			Repo:   repo,
			Email:  email,
		},
	}
	raw, err := json.Marshal(event)
	return event, raw, err
}

func correlationIDFromContext(ctx context.Context, fallback string) string {
	if id, ok := requestid.FromContext(ctx); ok {
		if _, err := uuid.Parse(id); err == nil {
			return id
		}
	}
	return fallback
}
