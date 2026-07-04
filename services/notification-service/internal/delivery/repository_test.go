package delivery

import (
	"encoding/json"
	"testing"
	"time"

	"subber/pkg/contracts"
)

func TestBuildDLQOutboxEvent_PreservesCorrelationID(t *testing.T) {
	payload := contracts.NotificationSendRequestedPayload{
		NotificationID: "notification-id",
		IdempotencyKey: "owner/repo:v1:user",
		RecipientEmail: "user@example.com",
		EmailHash:      "email-hash",
		Repo:           "owner/repo",
		Tag:            "v1",
		Message:        "hello",
	}
	occurredAt := time.Date(2026, 6, 24, 12, 0, 0, 0, time.UTC)

	event, err := buildDLQOutboxEvent(payload, "corr-1", occurredAt)
	if err != nil {
		t.Fatalf("buildDLQOutboxEvent() error = %v", err)
	}
	if event.Topic != contracts.TopicNotificationDLQ {
		t.Fatalf("topic = %q, want %q", event.Topic, contracts.TopicNotificationDLQ)
	}
	if event.CorrelationID != "corr-1" {
		t.Fatalf("correlation id = %q, want corr-1", event.CorrelationID)
	}

	var envelope contracts.Envelope[contracts.NotificationSendRequestedPayload]
	if err := json.Unmarshal(event.Payload, &envelope); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if envelope.CorrelationID != "corr-1" {
		t.Fatalf("envelope correlation id = %q, want corr-1", envelope.CorrelationID)
	}
	if envelope.Payload != payload {
		t.Fatalf("payload = %#v, want %#v", envelope.Payload, payload)
	}
}
