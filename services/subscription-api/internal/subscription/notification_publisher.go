package subscription

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"subber/pkg/contracts"
	"subber/pkg/outbox"
	"subber/pkg/requestid"
	"subber/services/subscription-api/internal/hash"
)

type OutboxNotificationPublisher struct {
	pool                   *pgxpool.Pool
	baseURL                string
	notificationServiceURL string
	client                 *http.Client
}

func NewOutboxNotificationPublisher(pool *pgxpool.Pool, baseURL, notificationServiceURL string) *OutboxNotificationPublisher {
	return &OutboxNotificationPublisher{
		pool:                   pool,
		baseURL:                baseURL,
		notificationServiceURL: strings.TrimRight(notificationServiceURL, "/"),
		client:                 &http.Client{Timeout: 5 * time.Second},
	}
}

func (p *OutboxNotificationPublisher) SendConfirmation(ctx context.Context, email, repo, token string) error {
	if p.notificationServiceURL == "" {
		return fmt.Errorf("notification service url is empty")
	}
	confirmURL := fmt.Sprintf("%s/api/confirm/%s", p.baseURL, token)
	body, err := json.Marshal(struct {
		Email      string `json:"email"`
		Repo       string `json:"repo"`
		ConfirmURL string `json:"confirm_url"`
	}{
		Email:      email,
		Repo:       repo,
		ConfirmURL: confirmURL,
	})
	if err != nil {
		return fmt.Errorf("marshal confirmation request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.notificationServiceURL+"/internal/notifications/confirmation", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build confirmation request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if correlationID, ok := requestid.FromContext(ctx); ok {
		req.Header.Set(requestid.Header, correlationID)
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("send confirmation request: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("send confirmation request: status %d", resp.StatusCode)
	}
	return nil
}

func (p *OutboxNotificationPublisher) PublishReleaseNotification(ctx context.Context, email, repo, tag, correlationID string) error {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin release notification: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

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
	return tx.Commit(ctx)
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
