package delivery

import (
	"context"
	"errors"
	"testing"
	"time"

	"subber/pkg/contracts"
)

type fakeStore struct {
	delivery      Delivery
	upsertErr     error
	markSentErr   error
	markFailedErr error
	markDeadErr   error
	markSentKey   string
	markFailedKey string
	markFailedAt  time.Time
	markDeadKey   string
}

func (s *fakeStore) UpsertPending(context.Context, contracts.NotificationSendRequestedPayload) (Delivery, error) {
	return s.delivery, s.upsertErr
}

func (s *fakeStore) MarkSent(_ context.Context, key string) error {
	s.markSentKey = key
	return s.markSentErr
}

func (s *fakeStore) MarkFailed(_ context.Context, key string, _ error, nextAttemptAt time.Time) error {
	s.markFailedKey = key
	s.markFailedAt = nextAttemptAt
	return s.markFailedErr
}

func (s *fakeStore) MarkDead(_ context.Context, key string, _ error) error {
	s.markDeadKey = key
	return s.markDeadErr
}

type fakeSender struct {
	err   error
	calls int
}

func (s *fakeSender) Send(string, string) error {
	s.calls++
	return s.err
}

type fakeRetryPublisher struct {
	err        error
	dlqCalls   int
}

func (p *fakeRetryPublisher) DeadLetter(context.Context, contracts.NotificationSendRequestedPayload, error) error {
	p.dlqCalls++
	return p.err
}

func notificationPayload() contracts.NotificationSendRequestedPayload {
	return contracts.NotificationSendRequestedPayload{
		NotificationID: "notification-id",
		IdempotencyKey: "owner/repo:v1:user",
		RecipientEmail: "user@example.com",
		EmailHash:      "user",
		Repo:           "owner/repo",
		Tag:            "v1",
		Message:        "hello",
	}
}

func TestProcess_SuccessMarksSent(t *testing.T) {
	store := &fakeStore{delivery: Delivery{Status: StatusPending}}
	sender := &fakeSender{}
	svc := NewService(store, sender, &fakeRetryPublisher{}, nil, 3, nil)

	if err := svc.Process(context.Background(), notificationPayload()); err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if sender.calls != 1 {
		t.Fatalf("sender calls = %d, want 1", sender.calls)
	}
	if store.markSentKey != "owner/repo:v1:user" {
		t.Fatalf("MarkSent key = %q", store.markSentKey)
	}
}

func TestProcess_SkipsSentAndDeadDeliveries(t *testing.T) {
	for _, status := range []string{StatusSent, StatusDead} {
		store := &fakeStore{delivery: Delivery{Status: status}}
		sender := &fakeSender{}
		svc := NewService(store, sender, &fakeRetryPublisher{}, nil, 3, nil)

		if err := svc.Process(context.Background(), notificationPayload()); err != nil {
			t.Fatalf("Process(%s) error = %v", status, err)
		}
		if sender.calls != 0 {
			t.Fatalf("sender calls = %d, want 0 for status %s", sender.calls, status)
		}
	}
}

func TestProcess_RetryableFailurePublishesRetry(t *testing.T) {
	store := &fakeStore{delivery: Delivery{Status: StatusPending, AttemptCount: 0}}
	sender := &fakeSender{err: errors.New("smtp down")}
	retries := &fakeRetryPublisher{}
	svc := NewService(store, sender, retries, nil, 3, []time.Duration{time.Minute, 10 * time.Minute})
	before := time.Now().UTC().Add(time.Minute)

	if err := svc.Process(context.Background(), notificationPayload()); err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if store.markFailedKey != "owner/repo:v1:user" {
		t.Fatalf("MarkFailed key = %q", store.markFailedKey)
	}
	if store.markFailedAt.Before(before) {
		t.Fatalf("nextAttemptAt = %s, want at or after %s", store.markFailedAt, before)
	}
}

func TestProcess_FinalFailurePublishesDLQ(t *testing.T) {
	store := &fakeStore{delivery: Delivery{Status: StatusFailed, AttemptCount: 2}}
	sender := &fakeSender{err: errors.New("smtp down")}
	retries := &fakeRetryPublisher{}
	svc := NewService(store, sender, retries, nil, 3, []time.Duration{time.Minute})

	if err := svc.Process(context.Background(), notificationPayload()); err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if store.markDeadKey != "owner/repo:v1:user" {
		t.Fatalf("MarkDead key = %q", store.markDeadKey)
	}
	if retries.dlqCalls != 1 {
		t.Fatalf("DLQ calls = %d, want 1", retries.dlqCalls)
	}
}

func TestProcess_ReturnsRepositoryErrors(t *testing.T) {
	repoErr := errors.New("db down")

	store := &fakeStore{upsertErr: repoErr}
	svc := NewService(store, &fakeSender{}, &fakeRetryPublisher{}, nil, 3, nil)
	if err := svc.Process(context.Background(), notificationPayload()); !errors.Is(err, repoErr) {
		t.Fatalf("upsert error = %v, want %v", err, repoErr)
	}

	store = &fakeStore{delivery: Delivery{Status: StatusPending}, markSentErr: repoErr}
	svc = NewService(store, &fakeSender{}, &fakeRetryPublisher{}, nil, 3, nil)
	if err := svc.Process(context.Background(), notificationPayload()); !errors.Is(err, repoErr) {
		t.Fatalf("mark sent error = %v, want %v", err, repoErr)
	}

	store = &fakeStore{delivery: Delivery{Status: StatusPending}, markFailedErr: repoErr}
	svc = NewService(store, &fakeSender{err: errors.New("smtp down")}, &fakeRetryPublisher{}, nil, 3, []time.Duration{time.Minute})
	if err := svc.Process(context.Background(), notificationPayload()); !errors.Is(err, repoErr) {
		t.Fatalf("mark failed error = %v, want %v", err, repoErr)
	}

	store = &fakeStore{delivery: Delivery{Status: StatusFailed, AttemptCount: 2}, markDeadErr: repoErr}
	svc = NewService(store, &fakeSender{err: errors.New("smtp down")}, &fakeRetryPublisher{}, nil, 3, []time.Duration{time.Minute})
	if err := svc.Process(context.Background(), notificationPayload()); !errors.Is(err, repoErr) {
		t.Fatalf("mark dead error = %v, want %v", err, repoErr)
	}
}

func TestProcess_ReturnsDeadLetterPublisherErrors(t *testing.T) {
	publishErr := errors.New("kafka down")

	store := &fakeStore{delivery: Delivery{Status: StatusFailed, AttemptCount: 2}}
	svc := NewService(store, &fakeSender{err: errors.New("smtp down")}, &fakeRetryPublisher{err: publishErr}, nil, 3, []time.Duration{time.Minute})
	if err := svc.Process(context.Background(), notificationPayload()); !errors.Is(err, publishErr) {
		t.Fatalf("dlq publish error = %v, want %v", err, publishErr)
	}
}

func TestRetryDelayUsesDefaultAndLastConfiguredDelay(t *testing.T) {
	svc := NewService(&fakeStore{}, &fakeSender{}, nil, nil, 3, nil)
	if got := svc.retryDelay(1); got != time.Minute {
		t.Fatalf("default retryDelay = %s, want 1m", got)
	}

	svc = NewService(&fakeStore{}, &fakeSender{}, nil, nil, 3, []time.Duration{time.Second})
	if got := svc.retryDelay(10); got != time.Second {
		t.Fatalf("retryDelay overflow = %s, want 1s", got)
	}
}
