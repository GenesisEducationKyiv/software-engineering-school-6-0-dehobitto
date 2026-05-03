package database

import (
	"context"
	"fmt"
	"log"

	"subber/models"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository provides data access methods for the subscriptions table.
type Repository struct {
	pool *pgxpool.Pool
}

// NewRepository creates a new Repository backed by the given connection pool.
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// SaveSubscription inserts or updates a subscription record in the database.
func (r *Repository) SaveSubscription(ctx context.Context, sub models.Subscription) error {
	query := `
		INSERT INTO subscriptions (email, repo, confirmed, last_seen_tag, token)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (email, repo) DO UPDATE 
		SET last_seen_tag = EXCLUDED.last_seen_tag;
	`

	_, err := r.pool.Exec(ctx, query, sub.Email, sub.Repo, sub.Confirmed, sub.LastSeenTag, sub.Token)
	if err != nil {
		log.Printf("Failed to save subscription for %s: %v", sub.Email, err)
		return err
	}

	log.Printf("Subscription saved for %s on %s", sub.Email, sub.Repo)
	return nil
}

// ConfirmSubscriptionByToken marks a subscription as confirmed by its unique token.
func (r *Repository) ConfirmSubscriptionByToken(ctx context.Context, token string) error {
	query := `
	UPDATE subscriptions
	SET confirmed = true
	WHERE token = $1
	`

	_, err := r.pool.Exec(ctx, query, token)
	if err != nil {
		return err
	}

	return nil
}

// Unsubscribe deletes a subscription identified by its unique token.
func (r *Repository) Unsubscribe(ctx context.Context, token string) error {
	query := `
	DELETE FROM subscriptions
	WHERE token = $1
	`

	result, err := r.pool.Exec(ctx, query, token)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("token not found")
	}

	return nil
}

// GetSubscriptions returns all confirmed subscriptions for the given email.
func (r *Repository) GetSubscriptions(ctx context.Context, email string) ([]models.Subscription, error) {
	query := `
		SELECT email, repo, confirmed, last_seen_tag 
		FROM subscriptions
		WHERE email = $1 AND confirmed = true
	`

	rows, err := r.pool.Query(ctx, query, email)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	subs := make([]models.Subscription, 0)
	for rows.Next() {
		var s models.Subscription

		err := rows.Scan(
			&s.Email,
			&s.Repo,
			&s.Confirmed,
			&s.LastSeenTag,
		)
		if err != nil {
			return nil, fmt.Errorf("scan failed: %w", err)
		}

		subs = append(subs, s)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return subs, nil
}

// SubscriptionExists checks whether a subscription already exists for the given email and repo.
func (r *Repository) SubscriptionExists(ctx context.Context, email, repo string) (bool, error) {
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM subscriptions WHERE email = $1 AND repo = $2)`

	err := r.pool.QueryRow(ctx, query, email, repo).Scan(&exists)
	if err != nil {
		return false, err
	}

	return exists, nil
}

// GetUniqueSubscriptions returns distinct repo/tag pairs from all confirmed subscriptions.
func (r *Repository) GetUniqueSubscriptions(ctx context.Context) ([]models.GitHubRelease, error) {
	query := `
		SELECT DISTINCT repo, last_seen_tag 
		FROM subscriptions
		WHERE confirmed = true
	`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("error on query: %w", err)
	}
	defer rows.Close()

	var subs []models.GitHubRelease
	for rows.Next() {
		var s models.GitHubRelease

		err := rows.Scan(
			&s.Repo,
			&s.LastSeenTag,
		)
		if err != nil {
			return nil, fmt.Errorf("error while scanning rows: %w", err)
		}

		subs = append(subs, s)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return subs, nil
}

// GetSubscribers returns distinct email addresses subscribed to the given repo.
func (r *Repository) GetSubscribers(ctx context.Context, repo string) ([]string, error) {
	query := `
		SELECT DISTINCT email 
		FROM subscriptions
		WHERE repo = $1 AND confirmed = true
	`

	rows, err := r.pool.Query(ctx, query, repo)
	if err != nil {
		return nil, fmt.Errorf("error on query: %w", err)
	}
	defer rows.Close()

	var subs []string
	for rows.Next() {
		var s string

		err := rows.Scan(&s)
		if err != nil {
			return nil, fmt.Errorf("error while scanning rows: %w", err)
		}

		subs = append(subs, s)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return subs, nil
}

// UpdateTags updates the last_seen_tag for all subscriptions of the given repo.
func (r *Repository) UpdateTags(ctx context.Context, repo models.GitHubRelease) error {
	query := `
	UPDATE subscriptions
	SET last_seen_tag = $1
	WHERE repo = $2
	`

	_, err := r.pool.Exec(ctx, query, repo.LastSeenTag, repo.Repo)
	if err != nil {
		return err
	}

	return nil
}
