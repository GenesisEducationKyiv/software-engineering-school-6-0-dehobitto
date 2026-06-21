package delivery

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/golang/mock/gomock"

	"subber/pkg/contracts"
)

type MockMessagePublisher struct {
	ctrl     *gomock.Controller
	recorder *MockMessagePublisherMockRecorder
}

type MockMessagePublisherMockRecorder struct {
	mock *MockMessagePublisher
}

func NewMockMessagePublisher(ctrl *gomock.Controller) *MockMessagePublisher {
	mock := &MockMessagePublisher{ctrl: ctrl}
	mock.recorder = &MockMessagePublisherMockRecorder{mock}
	return mock
}

func (m *MockMessagePublisher) EXPECT() *MockMessagePublisherMockRecorder {
	return m.recorder
}

func (m *MockMessagePublisher) Publish(ctx context.Context, topic, key string, value []byte) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Publish", ctx, topic, key, value)
	ret0, _ := ret[0].(error)
	return ret0
}

func (mr *MockMessagePublisherMockRecorder) Publish(ctx, topic, key, value interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Publish", reflect.TypeOf((*MockMessagePublisher)(nil).Publish), ctx, topic, key, value)
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
			ctrl := gomock.NewController(t)
			publisher := NewMockMessagePublisher(ctrl)
			retries := NewKafkaRetryPublisher(publisher)
			beforeRetry := time.Now().UTC().Add(tt.delay)
			var publishedValue []byte
			publisher.EXPECT().
				Publish(gomock.Any(), tt.topic, payload.EmailHash, gomock.Any()).
				DoAndReturn(func(_ context.Context, _ string, _ string, value []byte) error {
					publishedValue = value
					return nil
				})

			if err := retries.Retry(context.Background(), payload, tt.delay); err != nil {
				t.Fatalf("Retry() error = %v", err)
			}
			assertPublishedNotification(t, publishedValue, payload)
			assertPublishedNotBefore(t, publishedValue, beforeRetry)
		})
	}
}

func TestKafkaRetryPublisher_DeadLetterPublishesDLQ(t *testing.T) {
	payload := contracts.NotificationSendRequestedPayload{EmailHash: "email-hash", Message: "hello"}
	ctrl := gomock.NewController(t)
	publisher := NewMockMessagePublisher(ctrl)
	retries := NewKafkaRetryPublisher(publisher)
	var publishedValue []byte
	publisher.EXPECT().
		Publish(gomock.Any(), contracts.TopicNotificationDLQ, payload.EmailHash, gomock.Any()).
		DoAndReturn(func(_ context.Context, _ string, _ string, value []byte) error {
			publishedValue = value
			return nil
		})

	if err := retries.DeadLetter(context.Background(), payload, errors.New("smtp down")); err != nil {
		t.Fatalf("DeadLetter() error = %v", err)
	}
	assertPublishedNotification(t, publishedValue, payload)
}

func TestKafkaRetryPublisher_ReturnsPublishError(t *testing.T) {
	ctrl := gomock.NewController(t)
	publisher := NewMockMessagePublisher(ctrl)
	publishErr := errors.New("kafka down")
	payload := contracts.NotificationSendRequestedPayload{EmailHash: "email-hash"}
	publisher.EXPECT().Publish(gomock.Any(), contracts.TopicNotificationRetry1m, "email-hash", gomock.Any()).Return(publishErr)
	retries := NewKafkaRetryPublisher(publisher)
	err := retries.Retry(context.Background(), payload, time.Minute)
	if !errors.Is(err, publishErr) {
		t.Fatalf("Retry() error = %v, want %v", err, publishErr)
	}
}

func assertPublishedNotification(t *testing.T, value []byte, payload contracts.NotificationSendRequestedPayload) {
	t.Helper()
	var event contracts.Envelope[contracts.NotificationSendRequestedPayload]
	if err := json.Unmarshal(value, &event); err != nil {
		t.Fatalf("published value is not notification envelope: %v", err)
	}
	if event.EventType != contracts.EventNotificationRequested || event.Source != "notification-service" {
		t.Fatalf("metadata = %#v", event)
	}
	if event.Payload.EmailHash != payload.EmailHash || event.Payload.Message != payload.Message {
		t.Fatalf("payload = %#v, want %#v", event.Payload, payload)
	}
}

func assertPublishedNotBefore(t *testing.T, value []byte, earliest time.Time) {
	t.Helper()
	var event contracts.Envelope[contracts.NotificationSendRequestedPayload]
	if err := json.Unmarshal(value, &event); err != nil {
		t.Fatalf("published value is not notification envelope: %v", err)
	}
	if event.NotBefore == nil {
		t.Fatal("not_before is nil, want delayed retry timestamp")
	}
	if event.NotBefore.Before(earliest) {
		t.Fatalf("not_before = %s, want at or after %s", event.NotBefore, earliest)
	}
}
