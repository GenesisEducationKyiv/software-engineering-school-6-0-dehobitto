package delivery

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"subber/pkg/contracts"
)

type MessagePublisher interface {
	Publish(ctx context.Context, topic, key string, value []byte) error
}

type KafkaRetryPublisher struct {
	publisher MessagePublisher
}

func NewKafkaRetryPublisher(publisher MessagePublisher) *KafkaRetryPublisher {
	return &KafkaRetryPublisher{publisher: publisher}
}

func (p *KafkaRetryPublisher) Retry(ctx context.Context, payload contracts.NotificationSendRequestedPayload, delay time.Duration) error {
	topic := contracts.TopicNotificationRetry10m
	if delay <= time.Minute {
		topic = contracts.TopicNotificationRetry1m
	}
	notBefore := time.Now().UTC().Add(delay)
	return p.publish(ctx, topic, payload, &notBefore)
}

func (p *KafkaRetryPublisher) DeadLetter(ctx context.Context, payload contracts.NotificationSendRequestedPayload, _ error) error {
	return p.publish(ctx, contracts.TopicNotificationDLQ, payload, nil)
}

func (p *KafkaRetryPublisher) publish(ctx context.Context, topic string, payload contracts.NotificationSendRequestedPayload, notBefore *time.Time) error {
	eventID := uuid.NewString()
	occurredAt := time.Now().UTC()
	event := contracts.Envelope[contracts.NotificationSendRequestedPayload]{
		EventID:       eventID,
		EventType:     contracts.EventNotificationRequested,
		OccurredAt:    occurredAt,
		Source:        "notification-service",
		CorrelationID: eventID,
		NotBefore:     notBefore,
		Payload:       payload,
	}
	value, err := json.Marshal(event)
	if err != nil {
		return err
	}
	return p.publisher.Publish(ctx, topic, payload.EmailHash, value)
}
