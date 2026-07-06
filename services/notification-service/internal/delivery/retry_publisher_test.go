package delivery

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"subber/pkg/contracts"
)

type fakeMessagePublisher struct {
	err    error
	topic  string
	key    string
	value  []byte
	called bool
}

func (p *fakeMessagePublisher) Publish(_ context.Context, topic, key string, value []byte) error {
	p.called = true
	p.topic = topic
	p.key = key
	p.value = value
	return p.err
}

func TestKafkaRetryPublisher_RetryChoosesTopicAndPublishesEnvelope(t *testing.T) {
	payload := contracts.NotificationSendRequestedPayload{
		NotificationID: "notification-id",
		IdempotencyKey: "owner/repo:v1:user",
		RecipientEmail: "user@example.com",
		EmailHash:      "email-hash",
		Repo:           "owner/repo",
		Tag:            "v1",
		Message:        "hello",
	}

	tests := []struct {
		name  string
		delay time.Duration
		topic string
	}{
		{"short retry", time.Minute, contracts.TopicNotificationRetry1m},
		{"long retry", 10 * time.Minute, contracts.TopicNotificationRetry10m},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			publisher := &fakeMessagePublisher{}
			retries := NewKafkaRetryPublisher(publisher)
			beforeRetry := time.Now().UTC().Add(tt.delay)

			if err := retries.Retry(context.Background(), payload, tt.delay); err != nil {
				t.Fatalf("Retry() error = %v", err)
			}
			assertPublishedNotification(t, publisher, tt.topic, payload)
			assertPublishedNotBefore(t, publisher, beforeRetry)
		})
	}
}

func TestKafkaRetryPublisher_DeadLetterPublishesDLQ(t *testing.T) {
	payload := contracts.NotificationSendRequestedPayload{EmailHash: "email-hash", Message: "hello"}
	publisher := &fakeMessagePublisher{}
	retries := NewKafkaRetryPublisher(publisher)

	if err := retries.DeadLetter(context.Background(), payload, errors.New("smtp down")); err != nil {
		t.Fatalf("DeadLetter() error = %v", err)
	}
	assertPublishedNotification(t, publisher, contracts.TopicNotificationDLQ, payload)
}

func TestKafkaRetryPublisher_ReturnsPublishError(t *testing.T) {
	publishErr := errors.New("kafka down")
	retries := NewKafkaRetryPublisher(&fakeMessagePublisher{err: publishErr})
	err := retries.Retry(context.Background(), contracts.NotificationSendRequestedPayload{EmailHash: "email-hash"}, time.Minute)
	if !errors.Is(err, publishErr) {
		t.Fatalf("Retry() error = %v, want %v", err, publishErr)
	}
}

func assertPublishedNotification(t *testing.T, publisher *fakeMessagePublisher, topic string, payload contracts.NotificationSendRequestedPayload) {
	t.Helper()
	if !publisher.called {
		t.Fatal("publisher was not called")
	}
	if publisher.topic != topic {
		t.Fatalf("topic = %q, want %q", publisher.topic, topic)
	}
	if publisher.key != payload.EmailHash {
		t.Fatalf("key = %q, want %q", publisher.key, payload.EmailHash)
	}

	var event contracts.Envelope[contracts.NotificationSendRequestedPayload]
	if err := json.Unmarshal(publisher.value, &event); err != nil {
		t.Fatalf("published value is not notification envelope: %v", err)
	}
	if event.EventType != contracts.EventNotificationRequested || event.Source != "notification-service" {
		t.Fatalf("metadata = %#v", event)
	}
	if event.Payload.EmailHash != payload.EmailHash || event.Payload.Message != payload.Message {
		t.Fatalf("payload = %#v, want %#v", event.Payload, payload)
	}
}

func assertPublishedNotBefore(t *testing.T, publisher *fakeMessagePublisher, earliest time.Time) {
	t.Helper()
	var event contracts.Envelope[contracts.NotificationSendRequestedPayload]
	if err := json.Unmarshal(publisher.value, &event); err != nil {
		t.Fatalf("published value is not notification envelope: %v", err)
	}
	if event.NotBefore == nil {
		t.Fatal("not_before is nil, want delayed retry timestamp")
	}
	if event.NotBefore.Before(earliest) {
		t.Fatalf("not_before = %s, want at or after %s", event.NotBefore, earliest)
	}
}
