package subscription

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/jackc/pgx/v5"
)

type MockNotificationPublisher struct {
	ctrl     *gomock.Controller
	recorder *MockNotificationPublisherMockRecorder
}

type MockNotificationPublisherMockRecorder struct {
	mock *MockNotificationPublisher
}

func NewMockNotificationPublisher(ctrl *gomock.Controller) *MockNotificationPublisher {
	mock := &MockNotificationPublisher{ctrl: ctrl}
	mock.recorder = &MockNotificationPublisherMockRecorder{mock}
	return mock
}

func (m *MockNotificationPublisher) EXPECT() *MockNotificationPublisherMockRecorder {
	return m.recorder
}

func (m *MockNotificationPublisher) PublishConfirmationTx(ctx context.Context, tx pgx.Tx, email, repo, token string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "PublishConfirmationTx", ctx, tx, email, repo, token)
	ret0, _ := ret[0].(error)
	return ret0
}

func (mr *MockNotificationPublisherMockRecorder) PublishConfirmationTx(ctx, tx, email, repo, token interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "PublishConfirmationTx", reflect.TypeOf((*MockNotificationPublisher)(nil).PublishConfirmationTx), ctx, tx, email, repo, token)
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

func (m *MockStore) SaveSubscriptionWithConfirmation(ctx context.Context, sub Subscription, publisher NotificationPublisher) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SaveSubscriptionWithConfirmation", ctx, sub, publisher)
	ret0, _ := ret[0].(error)
	return ret0
}

func (mr *MockStoreMockRecorder) SaveSubscriptionWithConfirmation(ctx, sub, publisher interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SaveSubscriptionWithConfirmation", reflect.TypeOf((*MockStore)(nil).SaveSubscriptionWithConfirmation), ctx, sub, publisher)
}

func (m *MockStore) SubscriptionExists(ctx context.Context, email, repo string) (bool, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SubscriptionExists", ctx, email, repo)
	ret0, _ := ret[0].(bool)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

func (mr *MockStoreMockRecorder) SubscriptionExists(ctx, email, repo interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SubscriptionExists", reflect.TypeOf((*MockStore)(nil).SubscriptionExists), ctx, email, repo)
}

type MockGitHubClient struct {
	ctrl     *gomock.Controller
	recorder *MockGitHubClientMockRecorder
}

type MockGitHubClientMockRecorder struct {
	mock *MockGitHubClient
}

func NewMockGitHubClient(ctrl *gomock.Controller) *MockGitHubClient {
	mock := &MockGitHubClient{ctrl: ctrl}
	mock.recorder = &MockGitHubClientMockRecorder{mock}
	return mock
}

func (m *MockGitHubClient) EXPECT() *MockGitHubClientMockRecorder {
	return m.recorder
}

func (m *MockGitHubClient) CheckIfRepoExists(ctx context.Context, repo string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CheckIfRepoExists", ctx, repo)
	ret0, _ := ret[0].(error)
	return ret0
}

func (mr *MockGitHubClientMockRecorder) CheckIfRepoExists(ctx, repo interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CheckIfRepoExists", reflect.TypeOf((*MockGitHubClient)(nil).CheckIfRepoExists), ctx, repo)
}

func (m *MockGitHubClient) GetLatestTag(ctx context.Context, repo string) (string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetLatestTag", ctx, repo)
	ret0, _ := ret[0].(string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

func (mr *MockGitHubClientMockRecorder) GetLatestTag(ctx, repo interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetLatestTag", reflect.TypeOf((*MockGitHubClient)(nil).GetLatestTag), ctx, repo)
}

func TestSubscribe_SuccessSavesUnconfirmedAndPublishesConfirmation(t *testing.T) {
	ctrl := gomock.NewController(t)
	store := NewMockStore(ctrl)
	gh := NewMockGitHubClient(ctrl)
	notifications := NewMockNotificationPublisher(ctrl)
	var saved Subscription
	var notificationToken string
	gomock.InOrder(
		store.EXPECT().SubscriptionExists(gomock.Any(), "user@example.com", "owner/repo").Return(false, nil),
		gh.EXPECT().CheckIfRepoExists(gomock.Any(), "owner/repo").Return(nil),
		gh.EXPECT().GetLatestTag(gomock.Any(), "owner/repo").Return("v1.0.0", nil),
		store.EXPECT().
			SaveSubscriptionWithConfirmation(gomock.Any(), gomock.Any(), notifications).
			DoAndReturn(func(ctx context.Context, sub Subscription, publisher NotificationPublisher) error {
				saved = sub
				return publisher.PublishConfirmationTx(ctx, nil, sub.Email, sub.Repo, sub.Token)
			}),
		notifications.EXPECT().
			PublishConfirmationTx(gomock.Any(), nil, "user@example.com", "owner/repo", gomock.Any()).
			DoAndReturn(func(_ context.Context, _ pgx.Tx, _, _, token string) error {
				notificationToken = token
				return nil
			}),
	)
	svc := NewService(store, notifications, gh, nil)

	if err := svc.Subscribe(context.Background(), "user@example.com", "owner/repo"); err != nil {
		t.Fatalf("Subscribe() error = %v", err)
	}

	if saved.Confirmed {
		t.Fatal("subscription must start unconfirmed")
	}
	if saved.LastSeenTag != "v1.0.0" {
		t.Fatalf("LastSeenTag = %q, want v1.0.0", saved.LastSeenTag)
	}
	if notificationToken == "" || notificationToken != saved.Token {
		t.Fatalf("confirmation token = %q, saved token = %q", notificationToken, saved.Token)
	}
}

func TestSubscribe_AlreadySubscribedShortCircuits(t *testing.T) {
	ctrl := gomock.NewController(t)
	store := NewMockStore(ctrl)
	gh := NewMockGitHubClient(ctrl)
	notifications := NewMockNotificationPublisher(ctrl)
	store.EXPECT().SubscriptionExists(gomock.Any(), "user@example.com", "owner/repo").Return(true, nil)
	svc := NewService(store, notifications, gh, nil)

	err := svc.Subscribe(context.Background(), "user@example.com", "owner/repo")
	if !errors.Is(err, ErrAlreadySubscribed) {
		t.Fatalf("Subscribe() error = %v, want ErrAlreadySubscribed", err)
	}
}

func TestSubscribe_RepositoryCheckFailurePropagates(t *testing.T) {
	ctrl := gomock.NewController(t)
	store := NewMockStore(ctrl)
	gh := NewMockGitHubClient(ctrl)
	notifications := NewMockNotificationPublisher(ctrl)
	store.EXPECT().SubscriptionExists(gomock.Any(), "user@example.com", "owner/repo").Return(false, errors.New("db timeout"))
	svc := NewService(store, notifications, gh, nil)

	if err := svc.Subscribe(context.Background(), "user@example.com", "owner/repo"); err == nil {
		t.Fatal("expected repository error, got nil")
	}
}

func TestSubscribe_MapsGitHubErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want error
	}{
		{"not found", ErrGitHubNotFound, ErrRepoNotFound},
		{"rate limit", ErrGitHubAPILimit, ErrGitHubRateLimit},
		{"unavailable", errors.New("network"), ErrGitHubUnavailable},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			store := NewMockStore(ctrl)
			gh := NewMockGitHubClient(ctrl)
			notifications := NewMockNotificationPublisher(ctrl)
			store.EXPECT().SubscriptionExists(gomock.Any(), "user@example.com", "owner/repo").Return(false, nil)
			gh.EXPECT().CheckIfRepoExists(gomock.Any(), "owner/repo").Return(tt.err)
			svc := NewService(store, notifications, gh, nil)
			err := svc.Subscribe(context.Background(), "user@example.com", "owner/repo")
			if !errors.Is(err, tt.want) {
				t.Fatalf("Subscribe() error = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestSubscribe_SaveFailureDoesNotPublishConfirmation(t *testing.T) {
	ctrl := gomock.NewController(t)
	store := NewMockStore(ctrl)
	notifications := NewMockNotificationPublisher(ctrl)
	gh := NewMockGitHubClient(ctrl)
	saveErr := errors.New("db down")
	gomock.InOrder(
		store.EXPECT().SubscriptionExists(gomock.Any(), "user@example.com", "owner/repo").Return(false, nil),
		gh.EXPECT().CheckIfRepoExists(gomock.Any(), "owner/repo").Return(nil),
		gh.EXPECT().GetLatestTag(gomock.Any(), "owner/repo").Return("v1.0.0", nil),
		store.EXPECT().SaveSubscriptionWithConfirmation(gomock.Any(), gomock.Any(), notifications).Return(saveErr),
	)
	svc := NewService(store, notifications, gh, nil)

	if err := svc.Subscribe(context.Background(), "user@example.com", "owner/repo"); err == nil {
		t.Fatal("expected save error, got nil")
	}
}

func TestSubscribe_NotificationFailurePropagatesAfterSave(t *testing.T) {
	ctrl := gomock.NewController(t)
	store := NewMockStore(ctrl)
	notifications := NewMockNotificationPublisher(ctrl)
	gh := NewMockGitHubClient(ctrl)
	publishErr := errors.New("outbox down")
	gomock.InOrder(
		store.EXPECT().SubscriptionExists(gomock.Any(), "user@example.com", "owner/repo").Return(false, nil),
		gh.EXPECT().CheckIfRepoExists(gomock.Any(), "owner/repo").Return(nil),
		gh.EXPECT().GetLatestTag(gomock.Any(), "owner/repo").Return("v1.0.0", nil),
		store.EXPECT().
			SaveSubscriptionWithConfirmation(gomock.Any(), gomock.Any(), notifications).
			DoAndReturn(func(ctx context.Context, sub Subscription, publisher NotificationPublisher) error {
				return publisher.PublishConfirmationTx(ctx, nil, sub.Email, sub.Repo, sub.Token)
			}),
		notifications.EXPECT().PublishConfirmationTx(gomock.Any(), nil, "user@example.com", "owner/repo", gomock.Any()).Return(publishErr),
	)
	svc := NewService(store, notifications, gh, nil)

	if err := svc.Subscribe(context.Background(), "user@example.com", "owner/repo"); err == nil {
		t.Fatal("expected notification publisher error, got nil")
	}
}

func TestSubscribe_TagFetchFailureStillSaves(t *testing.T) {
	ctrl := gomock.NewController(t)
	store := NewMockStore(ctrl)
	notifications := NewMockNotificationPublisher(ctrl)
	gh := NewMockGitHubClient(ctrl)
	var saved Subscription
	gomock.InOrder(
		store.EXPECT().SubscriptionExists(gomock.Any(), "user@example.com", "owner/repo").Return(false, nil),
		gh.EXPECT().CheckIfRepoExists(gomock.Any(), "owner/repo").Return(nil),
		gh.EXPECT().GetLatestTag(gomock.Any(), "owner/repo").Return("", errors.New("timeout")),
		store.EXPECT().
			SaveSubscriptionWithConfirmation(gomock.Any(), gomock.Any(), notifications).
			DoAndReturn(func(_ context.Context, sub Subscription, _ NotificationPublisher) error {
				saved = sub
				return nil
			}),
	)
	svc := NewService(store, notifications, gh, nil)

	if err := svc.Subscribe(context.Background(), "user@example.com", "owner/repo"); err != nil {
		t.Fatalf("Subscribe() error = %v", err)
	}
	if saved.LastSeenTag != "" {
		t.Fatalf("LastSeenTag = %q, want empty", saved.LastSeenTag)
	}
}
