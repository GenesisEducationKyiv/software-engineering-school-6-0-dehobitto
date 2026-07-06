package subscription

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/golang/mock/gomock"

	"subber/pkg/contracts"
)

type MockSubscriberStore struct {
	ctrl     *gomock.Controller
	recorder *MockSubscriberStoreMockRecorder
}

type MockSubscriberStoreMockRecorder struct {
	mock *MockSubscriberStore
}

func NewMockSubscriberStore(ctrl *gomock.Controller) *MockSubscriberStore {
	mock := &MockSubscriberStore{ctrl: ctrl}
	mock.recorder = &MockSubscriberStoreMockRecorder{mock}
	return mock
}

func (m *MockSubscriberStore) EXPECT() *MockSubscriberStoreMockRecorder {
	return m.recorder
}

func (m *MockSubscriberStore) GetSubscribers(ctx context.Context, repo string) ([]string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetSubscribers", ctx, repo)
	ret0, _ := ret[0].([]string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

func (mr *MockSubscriberStoreMockRecorder) GetSubscribers(ctx, repo interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetSubscribers", reflect.TypeOf((*MockSubscriberStore)(nil).GetSubscribers), ctx, repo)
}

type MockReleaseNotificationPublisher struct {
	ctrl     *gomock.Controller
	recorder *MockReleaseNotificationPublisherMockRecorder
}

type MockReleaseNotificationPublisherMockRecorder struct {
	mock *MockReleaseNotificationPublisher
}

func NewMockReleaseNotificationPublisher(ctrl *gomock.Controller) *MockReleaseNotificationPublisher {
	mock := &MockReleaseNotificationPublisher{ctrl: ctrl}
	mock.recorder = &MockReleaseNotificationPublisherMockRecorder{mock}
	return mock
}

func (m *MockReleaseNotificationPublisher) EXPECT() *MockReleaseNotificationPublisherMockRecorder {
	return m.recorder
}

func (m *MockReleaseNotificationPublisher) PublishReleaseNotification(ctx context.Context, email, repo, tag, correlationID string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "PublishReleaseNotification", ctx, email, repo, tag, correlationID)
	ret0, _ := ret[0].(error)
	return ret0
}

func (mr *MockReleaseNotificationPublisherMockRecorder) PublishReleaseNotification(ctx, email, repo, tag, correlationID interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "PublishReleaseNotification", reflect.TypeOf((*MockReleaseNotificationPublisher)(nil).PublishReleaseNotification), ctx, email, repo, tag, correlationID)
}

func TestReleaseExpander_PublishesNotificationForEverySubscriber(t *testing.T) {
	ctrl := gomock.NewController(t)
	store := NewMockSubscriberStore(ctrl)
	publisher := NewMockReleaseNotificationPublisher(ctrl)
	gomock.InOrder(
		store.EXPECT().GetSubscribers(gomock.Any(), "owner/repo").Return([]string{"a@example.com", "b@example.com"}, nil),
		publisher.EXPECT().PublishReleaseNotification(gomock.Any(), "a@example.com", "owner/repo", "v2.0.0", "corr-1").Return(nil),
		publisher.EXPECT().PublishReleaseNotification(gomock.Any(), "b@example.com", "owner/repo", "v2.0.0", "corr-1").Return(nil),
	)
	expander := NewReleaseExpander(store, publisher)

	event := contracts.Envelope[contracts.ReleaseDetectedPayload]{
		CorrelationID: "corr-1",
		Payload: contracts.ReleaseDetectedPayload{
			Repo: "owner/repo",
			Tag:  "v2.0.0",
		},
	}
	if err := expander.Expand(context.Background(), event); err != nil {
		t.Fatalf("Expand() error = %v", err)
	}
}

func TestReleaseExpander_NoSubscribersDoesNothing(t *testing.T) {
	ctrl := gomock.NewController(t)
	store := NewMockSubscriberStore(ctrl)
	publisher := NewMockReleaseNotificationPublisher(ctrl)
	store.EXPECT().GetSubscribers(gomock.Any(), "owner/repo").Return(nil, nil)
	expander := NewReleaseExpander(store, publisher)

	err := expander.Expand(context.Background(), contracts.Envelope[contracts.ReleaseDetectedPayload]{
		Payload: contracts.ReleaseDetectedPayload{Repo: "owner/repo", Tag: "v1"},
	})
	if err != nil {
		t.Fatalf("Expand() error = %v", err)
	}
}

func TestReleaseExpander_ReturnsStoreAndPublisherErrors(t *testing.T) {
	ctrl := gomock.NewController(t)
	store := NewMockSubscriberStore(ctrl)
	publisher := NewMockReleaseNotificationPublisher(ctrl)
	storeErr := errors.New("db down")
	store.EXPECT().GetSubscribers(gomock.Any(), "").Return(nil, storeErr)
	expander := NewReleaseExpander(store, publisher)
	if err := expander.Expand(context.Background(), contracts.Envelope[contracts.ReleaseDetectedPayload]{}); !errors.Is(err, storeErr) {
		t.Fatalf("Expand() error = %v, want store error", err)
	}

	store = NewMockSubscriberStore(ctrl)
	publisher = NewMockReleaseNotificationPublisher(ctrl)
	publisherErr := errors.New("outbox down")
	gomock.InOrder(
		store.EXPECT().GetSubscribers(gomock.Any(), "owner/repo").Return([]string{"a@example.com"}, nil),
		publisher.EXPECT().PublishReleaseNotification(gomock.Any(), "a@example.com", "owner/repo", "v1", "").Return(publisherErr),
	)
	expander = NewReleaseExpander(store, publisher)
	err := expander.Expand(context.Background(), contracts.Envelope[contracts.ReleaseDetectedPayload]{
		Payload: contracts.ReleaseDetectedPayload{Repo: "owner/repo", Tag: "v1"},
	})
	if !errors.Is(err, publisherErr) {
		t.Fatalf("Expand() error = %v, want publisher error", err)
	}
}
