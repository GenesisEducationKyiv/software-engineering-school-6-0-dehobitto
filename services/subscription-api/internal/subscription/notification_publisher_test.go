package subscription

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"subber/pkg/contracts"
	"subber/services/subscription-api/internal/hash"
)

func TestBuildConfirmationNotification(t *testing.T) {
	occurredAt := time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)
	event, raw, err := buildConfirmationNotification(
		"http://localhost:8080",
		"USER@example.com",
		"owner/repo",
		"token-1",
		"notification-id",
		"correlation-id",
		occurredAt,
	)
	if err != nil {
		t.Fatalf("buildConfirmationNotification() error = %v", err)
	}

	if event.EventType != contracts.EventNotificationRequested || event.Source != "subscription-api" {
		t.Fatalf("unexpected envelope metadata: %#v", event)
	}
	if event.CorrelationID != "correlation-id" || event.OccurredAt != occurredAt {
		t.Fatalf("unexpected correlation/time: %#v", event)
	}
	if event.Payload.EmailHash != hash.EmailHash("USER@example.com") {
		t.Fatalf("EmailHash = %q", event.Payload.EmailHash)
	}
	if !strings.Contains(event.Payload.Message, "http://localhost:8080/api/confirm/token-1") {
		t.Fatalf("message missing confirmation URL: %q", event.Payload.Message)
	}
	if !strings.HasPrefix(event.Payload.IdempotencyKey, "confirmation:owner/repo:"+event.Payload.EmailHash+":") {
		t.Fatalf("idempotency key = %q", event.Payload.IdempotencyKey)
	}

	var decoded contracts.Envelope[contracts.NotificationSendRequestedPayload]
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("raw payload is not valid notification envelope: %v", err)
	}
	if decoded.Payload.Message != event.Payload.Message {
		t.Fatalf("decoded message = %q, want %q", decoded.Payload.Message, event.Payload.Message)
	}
}

func TestBuildReleaseNotification(t *testing.T) {
	event, raw, err := buildReleaseNotification("user@example.com", "owner/repo", "v2.0.0", "notification-id", "corr-1", time.Now().UTC())
	if err != nil {
		t.Fatalf("buildReleaseNotification() error = %v", err)
	}
	if event.Payload.IdempotencyKey != "owner/repo:v2.0.0:"+event.Payload.EmailHash {
		t.Fatalf("idempotency key = %q", event.Payload.IdempotencyKey)
	}
	if event.Payload.Message != "New release v2.0.0 for owner/repo!" {
		t.Fatalf("message = %q", event.Payload.Message)
	}
	if event.Payload.Tag != "v2.0.0" || event.Payload.Repo != "owner/repo" {
		t.Fatalf("payload = %#v", event.Payload)
	}
	if !json.Valid(raw) {
		t.Fatal("raw payload must be valid JSON")
	}
}
