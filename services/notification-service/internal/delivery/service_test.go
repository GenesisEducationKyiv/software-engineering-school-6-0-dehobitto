package delivery

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/golang/mock/gomock"

	"subber/pkg/contracts"
)

type MockEmailSender struct {
	ctrl     *gomock.Controller
	recorder *MockEmailSenderMockRecorder
}

type MockEmailSenderMockRecorder struct {
	mock *MockEmailSender
}

func NewMockEmailSender(ctrl *gomock.Controller) *MockEmailSender {
	mock := &MockEmailSender{ctrl: ctrl}
	mock.recorder = &MockEmailSenderMockRecorder{mock}
	return mock
}

func (m *MockEmailSender) EXPECT() *MockEmailSenderMockRecorder {
	return m.recorder
}

func (m *MockEmailSender) Send(to, body string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Send", to, body)
	ret0, _ := ret[0].(error)
	return ret0
}

func (mr *MockEmailSenderMockRecorder) Send(to, body interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Send", reflect.TypeOf((*MockEmailSender)(nil).Send), to, body)
}

type MockRetryPublisher struct {
	ctrl     *gomock.Controller
	recorder *MockRetryPublisherMockRecorder
}

type MockRetryPublisherMockRecorder struct {
	mock *MockRetryPublisher
}

func NewMockRetryPublisher(ctrl *gomock.Controller) *MockRetryPublisher {
	mock := &MockRetryPublisher{ctrl: ctrl}
	mock.recorder = &MockRetryPublisherMockRecorder{mock}
	return mock
}

func (m *MockRetryPublisher) EXPECT() *MockRetryPublisherMockRecorder {
	return m.recorder
}

func (m *MockRetryPublisher) DeadLetter(ctx context.Context, payload contracts.NotificationSendRequestedPayload, cause error) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DeadLetter", ctx, payload, cause)
	ret0, _ := ret[0].(error)
	return ret0
}

func (mr *MockRetryPublisherMockRecorder) DeadLetter(ctx, payload, cause interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeadLetter", reflect.TypeOf((*MockRetryPublisher)(nil).DeadLetter), ctx, payload, cause)
}

type MockStore struct {
	ctrl     *gomock.Controller
	recorder *MockStoreMockRecorder
}

type MockStoreMockRecorder struct {
	mock *MockStore
}

func NewMockStore(ctrl *gomock.Controller) *MockStore {
	mock := &MockStore{ctrl: ctrl}
	mock.recorder = &MockStoreMockRecorder{mock}
	return mock
}

func (m *MockStore) EXPECT() *MockStoreMockRecorder {
	return m.recorder
}

func (m *MockStore) MarkDead(ctx context.Context, idempotencyKey string, cause error) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "MarkDead", ctx, idempotencyKey, cause)
	ret0, _ := ret[0].(error)
	return ret0
}

func (mr *MockStoreMockRecorder) MarkDead(ctx, idempotencyKey, cause interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "MarkDead", reflect.TypeOf((*MockStore)(nil).MarkDead), ctx, idempotencyKey, cause)
}

func (m *MockStore) MarkFailed(ctx context.Context, idempotencyKey string, cause error, nextAttemptAt time.Time) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "MarkFailed", ctx, idempotencyKey, cause, nextAttemptAt)
	ret0, _ := ret[0].(error)
	return ret0
}

func (mr *MockStoreMockRecorder) MarkFailed(ctx, idempotencyKey, cause, nextAttemptAt interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "MarkFailed", reflect.TypeOf((*MockStore)(nil).MarkFailed), ctx, idempotencyKey, cause, nextAttemptAt)
}

func (m *MockStore) MarkSent(ctx context.Context, idempotencyKey string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "MarkSent", ctx, idempotencyKey)
	ret0, _ := ret[0].(error)
	return ret0
}

func (mr *MockStoreMockRecorder) MarkSent(ctx, idempotencyKey interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "MarkSent", reflect.TypeOf((*MockStore)(nil).MarkSent), ctx, idempotencyKey)
}

func (m *MockStore) UpsertPending(ctx context.Context, payload contracts.NotificationSendRequestedPayload) (Delivery, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "UpsertPending", ctx, payload)
	ret0, _ := ret[0].(Delivery)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

func (mr *MockStoreMockRecorder) UpsertPending(ctx, payload interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UpsertPending", reflect.TypeOf((*MockStore)(nil).UpsertPending), ctx, payload)
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
	ctrl := gomock.NewController(t)
	store := NewMockStore(ctrl)
	sender := NewMockEmailSender(ctrl)
	retries := NewMockRetryPublisher(ctrl)
	payload := notificationPayload()
	gomock.InOrder(
		store.EXPECT().UpsertPending(gomock.Any(), payload).Return(Delivery{Status: StatusPending}, nil),
		sender.EXPECT().Send("user@example.com", "hello").Return(nil),
		store.EXPECT().MarkSent(gomock.Any(), "owner/repo:v1:user").Return(nil),
	)
	svc := NewService(store, sender, retries, nil, 3, nil)

	if err := svc.Process(context.Background(), payload); err != nil {
		t.Fatalf("Process() error = %v", err)
	}
}

func TestProcess_SkipsSentAndDeadDeliveries(t *testing.T) {
	for _, status := range []string{StatusSent, StatusDead} {
		ctrl := gomock.NewController(t)
		store := NewMockStore(ctrl)
		sender := NewMockEmailSender(ctrl)
		retries := NewMockRetryPublisher(ctrl)
		payload := notificationPayload()
		store.EXPECT().UpsertPending(gomock.Any(), payload).Return(Delivery{Status: status}, nil)
		svc := NewService(store, sender, retries, nil, 3, nil)

		if err := svc.Process(context.Background(), payload); err != nil {
			t.Fatalf("Process(%s) error = %v", status, err)
		}
	}
}

func TestProcess_RetryableFailureMarksFailed(t *testing.T) {
	ctrl := gomock.NewController(t)
	store := NewMockStore(ctrl)
	sender := NewMockEmailSender(ctrl)
	retries := NewMockRetryPublisher(ctrl)
	payload := notificationPayload()
	smtpErr := errors.New("smtp down")
	var nextAttemptAt time.Time
	gomock.InOrder(
		store.EXPECT().UpsertPending(gomock.Any(), payload).Return(Delivery{Status: StatusPending, AttemptCount: 0}, nil),
		sender.EXPECT().Send("user@example.com", "hello").Return(smtpErr),
		store.EXPECT().MarkFailed(gomock.Any(), "owner/repo:v1:user", smtpErr, gomock.Any()).DoAndReturn(
			func(_ context.Context, _ string, _ error, attemptAt time.Time) error {
				nextAttemptAt = attemptAt
				return nil
			},
		),
	)
	svc := NewService(store, sender, retries, nil, 3, []time.Duration{time.Minute, 10 * time.Minute})
	before := time.Now().UTC().Add(time.Minute)

	if err := svc.Process(context.Background(), payload); err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if nextAttemptAt.Before(before) {
		t.Fatalf("nextAttemptAt = %s, want at or after %s", nextAttemptAt, before)
	}
}

func TestProcess_FinalFailurePublishesDLQ(t *testing.T) {
	ctrl := gomock.NewController(t)
	store := NewMockStore(ctrl)
	sender := NewMockEmailSender(ctrl)
	retries := NewMockRetryPublisher(ctrl)
	payload := notificationPayload()
	smtpErr := errors.New("smtp down")
	gomock.InOrder(
		store.EXPECT().UpsertPending(gomock.Any(), payload).Return(Delivery{Status: StatusFailed, AttemptCount: 2}, nil),
		sender.EXPECT().Send("user@example.com", "hello").Return(smtpErr),
		store.EXPECT().MarkDead(gomock.Any(), "owner/repo:v1:user", smtpErr).Return(nil),
		retries.EXPECT().DeadLetter(gomock.Any(), payload, smtpErr).Return(nil),
	)
	svc := NewService(store, sender, retries, nil, 3, []time.Duration{time.Minute})

	if err := svc.Process(context.Background(), payload); err != nil {
		t.Fatalf("Process() error = %v", err)
	}
}

func TestProcess_ReturnsRepositoryErrors(t *testing.T) {
	ctrl := gomock.NewController(t)
	repoErr := errors.New("db down")
	payload := notificationPayload()

	store := NewMockStore(ctrl)
	sender := NewMockEmailSender(ctrl)
	retries := NewMockRetryPublisher(ctrl)
	store.EXPECT().UpsertPending(gomock.Any(), payload).Return(Delivery{}, repoErr)
	svc := NewService(store, sender, retries, nil, 3, nil)
	if err := svc.Process(context.Background(), payload); !errors.Is(err, repoErr) {
		t.Fatalf("upsert error = %v, want %v", err, repoErr)
	}

	store = NewMockStore(ctrl)
	sender = NewMockEmailSender(ctrl)
	retries = NewMockRetryPublisher(ctrl)
	gomock.InOrder(
		store.EXPECT().UpsertPending(gomock.Any(), payload).Return(Delivery{Status: StatusPending}, nil),
		sender.EXPECT().Send("user@example.com", "hello").Return(nil),
		store.EXPECT().MarkSent(gomock.Any(), "owner/repo:v1:user").Return(repoErr),
	)
	svc = NewService(store, sender, retries, nil, 3, nil)
	if err := svc.Process(context.Background(), payload); !errors.Is(err, repoErr) {
		t.Fatalf("mark sent error = %v, want %v", err, repoErr)
	}

	store = NewMockStore(ctrl)
	sender = NewMockEmailSender(ctrl)
	retries = NewMockRetryPublisher(ctrl)
	smtpErr := errors.New("smtp down")
	gomock.InOrder(
		store.EXPECT().UpsertPending(gomock.Any(), payload).Return(Delivery{Status: StatusPending}, nil),
		sender.EXPECT().Send("user@example.com", "hello").Return(smtpErr),
		store.EXPECT().MarkFailed(gomock.Any(), "owner/repo:v1:user", smtpErr, gomock.Any()).Return(repoErr),
	)
	svc = NewService(store, sender, retries, nil, 3, []time.Duration{time.Minute})
	if err := svc.Process(context.Background(), payload); !errors.Is(err, repoErr) {
		t.Fatalf("mark failed error = %v, want %v", err, repoErr)
	}

	store = NewMockStore(ctrl)
	sender = NewMockEmailSender(ctrl)
	retries = NewMockRetryPublisher(ctrl)
	gomock.InOrder(
		store.EXPECT().UpsertPending(gomock.Any(), payload).Return(Delivery{Status: StatusFailed, AttemptCount: 2}, nil),
		sender.EXPECT().Send("user@example.com", "hello").Return(smtpErr),
		store.EXPECT().MarkDead(gomock.Any(), "owner/repo:v1:user", smtpErr).Return(repoErr),
	)
	svc = NewService(store, sender, retries, nil, 3, []time.Duration{time.Minute})
	if err := svc.Process(context.Background(), payload); !errors.Is(err, repoErr) {
		t.Fatalf("mark dead error = %v, want %v", err, repoErr)
	}
}

func TestProcess_ReturnsDeadLetterPublisherErrors(t *testing.T) {
	ctrl := gomock.NewController(t)
	publishErr := errors.New("kafka down")
	smtpErr := errors.New("smtp down")
	payload := notificationPayload()

	store := NewMockStore(ctrl)
	sender := NewMockEmailSender(ctrl)
	retries := NewMockRetryPublisher(ctrl)
	gomock.InOrder(
		store.EXPECT().UpsertPending(gomock.Any(), payload).Return(Delivery{Status: StatusFailed, AttemptCount: 2}, nil),
		sender.EXPECT().Send("user@example.com", "hello").Return(smtpErr),
		store.EXPECT().MarkDead(gomock.Any(), "owner/repo:v1:user", smtpErr).Return(nil),
		retries.EXPECT().DeadLetter(gomock.Any(), payload, smtpErr).Return(publishErr),
	)
	svc := NewService(store, sender, retries, nil, 3, []time.Duration{time.Minute})
	if err := svc.Process(context.Background(), payload); !errors.Is(err, publishErr) {
		t.Fatalf("dlq publish error = %v, want %v", err, publishErr)
	}
}

func TestRetryDelayUsesDefaultAndLastConfiguredDelay(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc := NewService(NewMockStore(ctrl), NewMockEmailSender(ctrl), nil, nil, 3, nil)
	if got := svc.retryDelay(1); got != time.Minute {
		t.Fatalf("default retryDelay = %s, want 1m", got)
	}

	svc = NewService(NewMockStore(ctrl), NewMockEmailSender(ctrl), nil, nil, 3, []time.Duration{time.Second})
	if got := svc.retryDelay(10); got != time.Second {
		t.Fatalf("retryDelay overflow = %s, want 1s", got)
	}
}
