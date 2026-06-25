package subscription

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/golang/mock/gomock"

	"subber/pkg/contracts"
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

func (m *MockNotificationPublisher) SendConfirmation(ctx context.Context, email, repo, token string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SendConfirmation", ctx, email, repo, token)
	ret0, _ := ret[0].(error)
	return ret0
}

func (mr *MockNotificationPublisherMockRecorder) SendConfirmation(ctx, email, repo, token interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SendConfirmation", reflect.TypeOf((*MockNotificationPublisher)(nil).SendConfirmation), ctx, email, repo, token)
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

func (m *MockStore) ConfirmSubscriptionByToken(ctx context.Context, token string) (ConfirmSubscriptionResult, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ConfirmSubscriptionByToken", ctx, token)
	ret0, _ := ret[0].(ConfirmSubscriptionResult)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

func (mr *MockStoreMockRecorder) ConfirmSubscriptionByToken(ctx, token interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ConfirmSubscriptionByToken", reflect.TypeOf((*MockStore)(nil).ConfirmSubscriptionByToken), ctx, token)
}

func (m *MockStore) RequestRepoWatchSaga(ctx context.Context, action, repo, email string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RequestRepoWatchSaga", ctx, action, repo, email)
	ret0, _ := ret[0].(error)
	return ret0
}

func (mr *MockStoreMockRecorder) RequestRepoWatchSaga(ctx, action, repo, email interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RequestRepoWatchSaga", reflect.TypeOf((*MockStore)(nil).RequestRepoWatchSaga), ctx, action, repo, email)
}

func (m *MockStore) DeleteUnconfirmedSubscription(ctx context.Context, email, repo, token string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DeleteUnconfirmedSubscription", ctx, email, repo, token)
	ret0, _ := ret[0].(error)
	return ret0
}

func (mr *MockStoreMockRecorder) DeleteUnconfirmedSubscription(ctx, email, repo, token interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeleteUnconfirmedSubscription", reflect.TypeOf((*MockStore)(nil).DeleteUnconfirmedSubscription), ctx, email, repo, token)
}

func (m *MockStore) SaveSubscription(ctx context.Context, sub Subscription) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SaveSubscription", ctx, sub)
	ret0, _ := ret[0].(error)
	return ret0
}

func (mr *MockStoreMockRecorder) SaveSubscription(ctx, sub interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SaveSubscription", reflect.TypeOf((*MockStore)(nil).SaveSubscription), ctx, sub)
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
			SaveSubscription(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, sub Subscription) error {
				saved = sub
				return nil
			}),
		notifications.EXPECT().
			SendConfirmation(gomock.Any(), "user@example.com", "owner/repo", gomock.Any()).
			DoAndReturn(func(_ context.Context, _, _, token string) error {
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
		store.EXPECT().SaveSubscription(gomock.Any(), gomock.Any()).Return(saveErr),
	)
	svc := NewService(store, notifications, gh, nil)

	if err := svc.Subscribe(context.Background(), "user@example.com", "owner/repo"); err == nil {
		t.Fatal("expected save error, got nil")
	}
}

func TestSubscribe_NotificationFailureCompensatesSubscription(t *testing.T) {
	ctrl := gomock.NewController(t)
	store := NewMockStore(ctrl)
	notifications := NewMockNotificationPublisher(ctrl)
	gh := NewMockGitHubClient(ctrl)
	publishErr := errors.New("notification down")
	var saved Subscription
	gomock.InOrder(
		store.EXPECT().SubscriptionExists(gomock.Any(), "user@example.com", "owner/repo").Return(false, nil),
		gh.EXPECT().CheckIfRepoExists(gomock.Any(), "owner/repo").Return(nil),
		gh.EXPECT().GetLatestTag(gomock.Any(), "owner/repo").Return("v1.0.0", nil),
		store.EXPECT().
			SaveSubscription(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, sub Subscription) error {
				saved = sub
				return nil
			}),
		notifications.EXPECT().SendConfirmation(gomock.Any(), "user@example.com", "owner/repo", gomock.Any()).Return(publishErr),
		store.EXPECT().DeleteUnconfirmedSubscription(gomock.Any(), "user@example.com", "owner/repo", gomock.Any()).DoAndReturn(
			func(_ context.Context, _, _, token string) error {
				if token != saved.Token {
					t.Fatalf("compensation token = %q, want %q", token, saved.Token)
				}
				return nil
			},
		),
	)
	svc := NewService(store, notifications, gh, nil)

	if err := svc.Subscribe(context.Background(), "user@example.com", "owner/repo"); !errors.Is(err, ErrConfirmationUnavailable) {
		t.Fatalf("Subscribe() error = %v, want ErrConfirmationUnavailable", err)
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
			SaveSubscription(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, sub Subscription) error {
				saved = sub
				return nil
			}),
		notifications.EXPECT().SendConfirmation(gomock.Any(), "user@example.com", "owner/repo", gomock.Any()).Return(nil),
	)
	svc := NewService(store, notifications, gh, nil)

	if err := svc.Subscribe(context.Background(), "user@example.com", "owner/repo"); err != nil {
		t.Fatalf("Subscribe() error = %v", err)
	}
	if saved.LastSeenTag != "" {
		t.Fatalf("LastSeenTag = %q, want empty", saved.LastSeenTag)
	}
}

func TestConfirmSubscription_StartsWatchSagaForFirstConfirmedSubscriber(t *testing.T) {
	ctrl := gomock.NewController(t)
	store := NewMockStore(ctrl)
	notifications := NewMockNotificationPublisher(ctrl)
	gh := NewMockGitHubClient(ctrl)
	token := "token-1"
	gomock.InOrder(
		store.EXPECT().
			ConfirmSubscriptionByToken(gomock.Any(), token).
			Return(ConfirmSubscriptionResult{
				Repo:              "owner/repo",
				Email:             "user@example.com",
				WasFirstConfirmed: true,
			}, nil),
		store.EXPECT().
			RequestRepoWatchSaga(gomock.Any(), contracts.RepoWatchActionStart, "owner/repo", "user@example.com").
			Return(nil),
	)
	svc := NewService(store, notifications, gh, nil)

	if err := svc.ConfirmSubscriptionByToken(context.Background(), token); err != nil {
		t.Fatalf("ConfirmSubscriptionByToken() error = %v", err)
	}
}

func TestConfirmSubscription_DoesNotStartWatchSagaForAlreadyWatchedRepo(t *testing.T) {
	ctrl := gomock.NewController(t)
	store := NewMockStore(ctrl)
	notifications := NewMockNotificationPublisher(ctrl)
	gh := NewMockGitHubClient(ctrl)
	token := "token-1"
	store.EXPECT().
		ConfirmSubscriptionByToken(gomock.Any(), token).
		Return(ConfirmSubscriptionResult{
			Repo:              "owner/repo",
			Email:             "user@example.com",
			WasFirstConfirmed: false,
		}, nil)
	svc := NewService(store, notifications, gh, nil)

	if err := svc.ConfirmSubscriptionByToken(context.Background(), token); err != nil {
		t.Fatalf("ConfirmSubscriptionByToken() error = %v", err)
	}
}
