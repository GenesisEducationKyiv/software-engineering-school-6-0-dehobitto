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
	dueRetries    []ScheduledRetry
	upsertErr     error
	fetchDueErr   error
	markSentErr   error
	markFailedErr error
	markDeadErr   error
	markSentKey   string
	markFailedKey string
	markDeadKey   string
	markDeadCID   string
	markFailedAt  time.Time
}

func (s *fakeStore) UpsertPending(_ context.Context, _ contracts.NotificationSendRequestedPayload, correlationID string) (Delivery, error) {
	if s.delivery.CorrelationID == "" {
		s.delivery.CorrelationID = correlationID
	}
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

func (s *fakeStore) MarkDead(_ context.Context, payload contracts.NotificationSendRequestedPayload, correlationID string, _ error) error {
	s.markDeadKey = payload.IdempotencyKey
	s.markDeadCID = correlationID
	return s.markDeadErr
}

func (s *fakeStore) FetchDueRetries(context.Context, int) ([]ScheduledRetry, error) {
	return s.dueRetries, s.fetchDueErr
}

type fakeSender struct {
	err   error
	calls int
}

func (s *fakeSender) Send(string, string) error {
	s.calls++
	return s.err
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
	svc := NewService(store, sender, nil, 3, nil)

	if err := svc.Process(context.Background(), notificationPayload(), "corr-1"); err != nil {
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
		svc := NewService(store, sender, nil, 3, nil)

		if err := svc.Process(context.Background(), notificationPayload(), "corr-1"); err != nil {
			t.Fatalf("Process(%s) error = %v", status, err)
		}
		if sender.calls != 0 {
			t.Fatalf("sender calls = %d, want 0 for status %s", sender.calls, status)
		}
	}
}

func TestProcess_RetryableFailureSchedulesRetry(t *testing.T) {
	store := &fakeStore{delivery: Delivery{Status: StatusPending, AttemptCount: 0}}
	sender := &fakeSender{err: errors.New("smtp down")}
	svc := NewService(store, sender, nil, 3, []time.Duration{time.Minute, 10 * time.Minute})

	if err := svc.Process(context.Background(), notificationPayload(), "corr-1"); err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if store.markFailedKey != "owner/repo:v1:user" {
		t.Fatalf("MarkFailed key = %q", store.markFailedKey)
	}
	if time.Until(store.markFailedAt) <= 0 || time.Until(store.markFailedAt) > 2*time.Minute {
		t.Fatalf("next attempt at = %s, want scheduled in the future", store.markFailedAt)
	}
}

func TestProcess_FinalFailureMarksDeadWithOriginalCorrelationID(t *testing.T) {
	store := &fakeStore{delivery: Delivery{Status: StatusFailed, AttemptCount: 2, CorrelationID: "corr-1"}}
	sender := &fakeSender{err: errors.New("smtp down")}
	svc := NewService(store, sender, nil, 3, []time.Duration{time.Minute})

	if err := svc.Process(context.Background(), notificationPayload(), "ignored-corr"); err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if store.markDeadKey != "owner/repo:v1:user" {
		t.Fatalf("MarkDead key = %q", store.markDeadKey)
	}
	if store.markDeadCID != "corr-1" {
		t.Fatalf("MarkDead correlation = %q, want corr-1", store.markDeadCID)
	}
}

func TestProcess_ReturnsRepositoryErrors(t *testing.T) {
	repoErr := errors.New("db down")

	store := &fakeStore{upsertErr: repoErr}
	svc := NewService(store, &fakeSender{}, nil, 3, nil)
	if err := svc.Process(context.Background(), notificationPayload(), "corr-1"); !errors.Is(err, repoErr) {
		t.Fatalf("upsert error = %v, want %v", err, repoErr)
	}

	store = &fakeStore{delivery: Delivery{Status: StatusPending}, markSentErr: repoErr}
	svc = NewService(store, &fakeSender{}, nil, 3, nil)
	if err := svc.Process(context.Background(), notificationPayload(), "corr-1"); !errors.Is(err, repoErr) {
		t.Fatalf("mark sent error = %v, want %v", err, repoErr)
	}

	store = &fakeStore{delivery: Delivery{Status: StatusPending}, markFailedErr: repoErr}
	svc = NewService(store, &fakeSender{err: errors.New("smtp down")}, nil, 3, []time.Duration{time.Minute})
	if err := svc.Process(context.Background(), notificationPayload(), "corr-1"); !errors.Is(err, repoErr) {
		t.Fatalf("mark failed error = %v, want %v", err, repoErr)
	}

	store = &fakeStore{delivery: Delivery{Status: StatusFailed, AttemptCount: 2}, markDeadErr: repoErr}
	svc = NewService(store, &fakeSender{err: errors.New("smtp down")}, nil, 3, []time.Duration{time.Minute})
	if err := svc.Process(context.Background(), notificationPayload(), "corr-1"); !errors.Is(err, repoErr) {
		t.Fatalf("mark dead error = %v, want %v", err, repoErr)
	}
}

func TestRetryDelayUsesDefaultAndLastConfiguredDelay(t *testing.T) {
	svc := NewService(&fakeStore{}, &fakeSender{}, nil, 3, nil)
	if got := svc.retryDelay(1); got != time.Minute {
		t.Fatalf("default retryDelay = %s, want 1m", got)
	}

	svc = NewService(&fakeStore{}, &fakeSender{}, nil, 3, []time.Duration{time.Second})
	if got := svc.retryDelay(10); got != time.Second {
		t.Fatalf("retryDelay overflow = %s, want 1s", got)
	}
}

type fakeRetryProcessor struct {
	err      error
	payloads []ScheduledRetry
}

func (p *fakeRetryProcessor) Process(_ context.Context, payload contracts.NotificationSendRequestedPayload, correlationID string) error {
	p.payloads = append(p.payloads, ScheduledRetry{Payload: payload, CorrelationID: correlationID})
	return p.err
}

func TestRetryRelay_ProcessesDueRetries(t *testing.T) {
	store := &fakeStore{dueRetries: []ScheduledRetry{{Payload: notificationPayload(), CorrelationID: "corr-1"}}}
	processor := &fakeRetryProcessor{}
	relay := NewRetryRelay(store, processor, 10, time.Second)

	if err := relay.ProcessOnce(context.Background()); err != nil {
		t.Fatalf("ProcessOnce() error = %v", err)
	}
	if len(processor.payloads) != 1 {
		t.Fatalf("processed payloads = %d, want 1", len(processor.payloads))
	}
	if processor.payloads[0].CorrelationID != "corr-1" {
		t.Fatalf("processed correlation = %q, want corr-1", processor.payloads[0].CorrelationID)
	}
}

func TestRetryRelay_ReturnsStoreAndProcessorErrors(t *testing.T) {
	storeErr := errors.New("db down")
	relay := NewRetryRelay(&fakeStore{fetchDueErr: storeErr}, &fakeRetryProcessor{}, 10, time.Second)
	if err := relay.ProcessOnce(context.Background()); !errors.Is(err, storeErr) {
		t.Fatalf("ProcessOnce() error = %v, want %v", err, storeErr)
	}

	processErr := errors.New("smtp down")
	relay = NewRetryRelay(&fakeStore{dueRetries: []ScheduledRetry{{Payload: notificationPayload(), CorrelationID: "corr-1"}}}, &fakeRetryProcessor{err: processErr}, 10, time.Second)
	if err := relay.ProcessOnce(context.Background()); !errors.Is(err, processErr) {
		t.Fatalf("ProcessOnce() error = %v, want %v", err, processErr)
	}
}
