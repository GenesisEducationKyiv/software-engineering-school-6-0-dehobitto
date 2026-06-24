package subscription

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
	"subber/pkg/requestid"
	"subber/services/subscription-api/internal/hash"
)

type OutboxNotificationPublisher struct {
	pool    *pgxpool.Pool
	baseURL string
}

func NewOutboxNotificationPublisher(pool *pgxpool.Pool, baseURL string) *OutboxNotificationPublisher {
	return &OutboxNotificationPublisher{pool: pool, baseURL: baseURL}
}

func (p *OutboxNotificationPublisher) PublishConfirmation(ctx context.Context, email, repo, token string) error {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin confirmation notification: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	if err := p.PublishConfirmationTx(ctx, tx, email, repo, token); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (p *OutboxNotificationPublisher) PublishConfirmationTx(ctx context.Context, tx pgx.Tx, email, repo, token string) error {
	notificationID := uuid.NewString()
	correlationID := correlationIDFromRequest(ctx, notificationID)
	event, raw, err := buildConfirmationNotification(p.baseURL, email, repo, token, notificationID, correlationID, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("marshal confirmation notification: %w", err)
	}
	if err := outbox.InsertTx(ctx, tx, outbox.Event{
		EventID:       notificationID,
		EventType:     contracts.EventNotificationRequested,
		OccurredAt:    event.OccurredAt,
		Source:        "subscription-api",
		CorrelationID: correlationID,
		Topic:         contracts.TopicNotificationCommands,
		KafkaKey:      event.Payload.EmailHash,
		Payload:       raw,
	}); err != nil {
		return err
	}
	return nil
}

func (p *OutboxNotificationPublisher) PublishReleaseNotification(ctx context.Context, email, repo, tag, correlationID string) error {
	return p.PublishReleaseNotifications(ctx, []string{email}, repo, tag, correlationID)
}

func (p *OutboxNotificationPublisher) PublishReleaseNotifications(ctx context.Context, emails []string, repo, tag, correlationID string) error {
	if len(emails) == 0 {
		return nil
	}

	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin release notifications: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	for _, email := range emails {
		notificationID := uuid.NewString()
		event, raw, err := buildReleaseNotification(email, repo, tag, notificationID, correlationID, time.Now().UTC())
		if err != nil {
			return fmt.Errorf("marshal release notification: %w", err)
		}
		if err := outbox.InsertTx(ctx, tx, outbox.Event{
			EventID:       notificationID,
			EventType:     contracts.EventNotificationRequested,
			OccurredAt:    event.OccurredAt,
			Source:        "subscription-api",
			CorrelationID: correlationID,
			Topic:         contracts.TopicNotificationCommands,
			KafkaKey:      event.Payload.EmailHash,
			Payload:       raw,
		}); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func buildConfirmationNotification(baseURL, email, repo, token, notificationID, correlationID string, occurredAt time.Time) (contracts.Envelope[contracts.NotificationSendRequestedPayload], []byte, error) {
	confirmURL := fmt.Sprintf("%s/api/confirm/%s", baseURL, token)
	message := fmt.Sprintf("Welcome! Please confirm your subscription to GitHub repository updates by clicking here: %s", confirmURL)
	emailHash := hash.EmailHash(email)
	event := contracts.Envelope[contracts.NotificationSendRequestedPayload]{
		EventID:       notificationID,
		EventType:     contracts.EventNotificationRequested,
		OccurredAt:    occurredAt,
		Source:        "subscription-api",
		CorrelationID: correlationID,
		Payload: contracts.NotificationSendRequestedPayload{
			NotificationID: notificationID,
			IdempotencyKey: fmt.Sprintf("confirmation:%s:%s:%s", repo, emailHash, hash.TextHash(message)),
			RecipientEmail: email,
			EmailHash:      emailHash,
			Repo:           repo,
			Message:        message,
		},
	}
	raw, err := json.Marshal(event)
	return event, raw, err
}

func buildReleaseNotification(email, repo, tag, notificationID, correlationID string, occurredAt time.Time) (contracts.Envelope[contracts.NotificationSendRequestedPayload], []byte, error) {
	emailHash := hash.EmailHash(email)
	event := contracts.Envelope[contracts.NotificationSendRequestedPayload]{
		EventID:       notificationID,
		EventType:     contracts.EventNotificationRequested,
		OccurredAt:    occurredAt,
		Source:        "subscription-api",
		CorrelationID: correlationID,
		Payload: contracts.NotificationSendRequestedPayload{
			NotificationID: notificationID,
			IdempotencyKey: fmt.Sprintf("%s:%s:%s", repo, tag, emailHash),
			RecipientEmail: email,
			EmailHash:      emailHash,
			Repo:           repo,
			Tag:            tag,
			Message:        fmt.Sprintf("New release %s for %s!", tag, repo),
		},
	}
	raw, err := json.Marshal(event)
	return event, raw, err
}

func correlationIDFromRequest(ctx context.Context, fallback string) string {
	if id, ok := requestid.FromContext(ctx); ok {
		if _, err := uuid.Parse(id); err == nil {
			return id
		}
	}
	return fallback
}
