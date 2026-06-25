package scanner

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/golang/mock/gomock"

	"subber/pkg/contracts"
	"subber/pkg/kafka"
)

type MockReleaseProvider struct {
	ctrl     *gomock.Controller
	recorder *MockReleaseProviderMockRecorder
}

type MockReleaseProviderMockRecorder struct {
	mock *MockReleaseProvider
}

func NewMockReleaseProvider(ctrl *gomock.Controller) *MockReleaseProvider {
	mock := &MockReleaseProvider{ctrl: ctrl}
	mock.recorder = &MockReleaseProviderMockRecorder{mock}
	return mock
}

func (m *MockReleaseProvider) EXPECT() *MockReleaseProviderMockRecorder {
	return m.recorder
}

func (m *MockReleaseProvider) GetLatestTag(ctx context.Context, repo string) (string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetLatestTag", ctx, repo)
	ret0, _ := ret[0].(string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

func (mr *MockReleaseProviderMockRecorder) GetLatestTag(ctx, repo interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetLatestTag", reflect.TypeOf((*MockReleaseProvider)(nil).GetLatestTag), ctx, repo)
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

func (m *MockStore) ApplyWatchCommand(ctx context.Context, payload contracts.RepoWatchCommandPayload, correlationID string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ApplyWatchCommand", ctx, payload, correlationID)
	ret0, _ := ret[0].(error)
	return ret0
}

func (mr *MockStoreMockRecorder) ApplyWatchCommand(ctx, payload, correlationID interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ApplyWatchCommand", reflect.TypeOf((*MockStore)(nil).ApplyWatchCommand), ctx, payload, correlationID)
}

func (m *MockStore) ClaimDue(ctx context.Context, limit int, nextScanIn time.Duration) ([]WatchedRepo, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ClaimDue", ctx, limit, nextScanIn)
	ret0, _ := ret[0].([]WatchedRepo)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

func (mr *MockStoreMockRecorder) ClaimDue(ctx, limit, nextScanIn interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ClaimDue", reflect.TypeOf((*MockStore)(nil).ClaimDue), ctx, limit, nextScanIn)
}

func (m *MockStore) MarkReleaseDetected(ctx context.Context, repo, tag, correlationID string) (bool, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "MarkReleaseDetected", ctx, repo, tag, correlationID)
	ret0, _ := ret[0].(bool)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

func (mr *MockStoreMockRecorder) MarkReleaseDetected(ctx, repo, tag, correlationID interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "MarkReleaseDetected", reflect.TypeOf((*MockStore)(nil).MarkReleaseDetected), ctx, repo, tag, correlationID)
}

func TestHandleWatchlistCommand_StartAndStop(t *testing.T) {
	ctrl := gomock.NewController(t)
	store := NewMockStore(ctrl)
	releases := NewMockReleaseProvider(ctrl)
	svc := NewService(store, releases, nil, 10, time.Minute)

	start := contracts.Envelope[contracts.RepoWatchCommandPayload]{
		EventType:     contracts.EventStartWatchingRepo,
		CorrelationID: "corr-1",
		Payload: contracts.RepoWatchCommandPayload{
			SagaID: "saga-1",
			Action: contracts.RepoWatchActionStart,
			Repo:   "owner/repo",
		},
	}
	startRaw, _ := json.Marshal(start)
	store.EXPECT().
		ApplyWatchCommand(gomock.Any(), start.Payload, "corr-1").
		Return(nil)
	if err := svc.HandleWatchlistCommand(context.Background(), startRaw); err != nil {
		t.Fatalf("HandleWatchlistCommand(start) error = %v", err)
	}

	stop := contracts.Envelope[contracts.RepoWatchCommandPayload]{
		EventType:     contracts.EventStopWatchingRepo,
		CorrelationID: "corr-2",
		Payload: contracts.RepoWatchCommandPayload{
			SagaID: "saga-2",
			Action: contracts.RepoWatchActionStop,
			Repo:   "owner/repo",
		},
	}
	stopRaw, _ := json.Marshal(stop)
	store.EXPECT().
		ApplyWatchCommand(gomock.Any(), stop.Payload, "corr-2").
		Return(nil)
	if err := svc.HandleWatchlistCommand(context.Background(), stopRaw); err != nil {
		t.Fatalf("HandleWatchlistCommand(stop) error = %v", err)
	}
}

func TestHandleWatchlistCommand_ReturnsDecodeUnsupportedAndStoreErrors(t *testing.T) {
	ctrl := gomock.NewController(t)
	store := NewMockStore(ctrl)
	releases := NewMockReleaseProvider(ctrl)
	svc := NewService(store, releases, nil, 10, time.Minute)
	if err := svc.HandleWatchlistCommand(context.Background(), []byte("not-json")); !errors.Is(err, kafka.ErrNonRetryable) {
		t.Fatalf("decode error = %v, want ErrNonRetryable", err)
	}

	unsupported := contracts.Envelope[contracts.RepoWatchCommandPayload]{
		EventType: "UnknownEvent",
		Payload:   contracts.RepoWatchCommandPayload{SagaID: "saga-1", Action: contracts.RepoWatchActionStart, Repo: "owner/repo"},
	}
	rawUnsupported, _ := json.Marshal(unsupported)
	if err := svc.HandleWatchlistCommand(context.Background(), rawUnsupported); err == nil {
		t.Fatal("expected unsupported event error, got nil")
	}

	storeErr := errors.New("db down")
	start := contracts.Envelope[contracts.RepoWatchCommandPayload]{
		EventType: contracts.EventStartWatchingRepo,
		Payload:   contracts.RepoWatchCommandPayload{SagaID: "saga-1", Action: contracts.RepoWatchActionStart, Repo: "owner/repo"},
	}
	rawStart, _ := json.Marshal(start)
	store.EXPECT().
		ApplyWatchCommand(gomock.Any(), start.Payload, "").
		Return(storeErr)
	if err := svc.HandleWatchlistCommand(context.Background(), rawStart); !errors.Is(err, storeErr) {
		t.Fatalf("store error = %v, want %v", err, storeErr)
	}
}

func TestScanOnce_ReturnsClaimError(t *testing.T) {
	ctrl := gomock.NewController(t)
	claimErr := errors.New("claim failed")
	store := NewMockStore(ctrl)
	releases := NewMockReleaseProvider(ctrl)
	store.EXPECT().
		ClaimDue(gomock.Any(), 10, time.Minute).
		Return(nil, claimErr)
	svc := NewService(store, releases, nil, 10, time.Minute)
	if err := svc.ScanOnce(context.Background()); !errors.Is(err, claimErr) {
		t.Fatalf("ScanOnce() error = %v, want %v", err, claimErr)
	}
}

func TestScanOnce_ClaimsConfiguredBatchAndPublishesNewRelease(t *testing.T) {
	ctrl := gomock.NewController(t)
	store := NewMockStore(ctrl)
	releases := NewMockReleaseProvider(ctrl)
	store.EXPECT().
		ClaimDue(gomock.Any(), 25, 45*time.Second).
		Return([]WatchedRepo{{Repo: "owner/repo", LastSeenTag: "v1.0.0"}}, nil)
	releases.EXPECT().
		GetLatestTag(gomock.Any(), "owner/repo").
		Return("v2.0.0", nil)
	store.EXPECT().
		MarkReleaseDetected(gomock.Any(), "owner/repo", "v2.0.0", gomock.Any()).
		Return(true, nil)
	svc := NewService(store, releases, nil, 25, 45*time.Second)

	if err := svc.ScanOnce(context.Background()); err != nil {
		t.Fatalf("ScanOnce() error = %v", err)
	}
}

func TestScanOnce_DoesNotPublishUnchangedOrErroredTag(t *testing.T) {
	tests := []struct {
		name string
		tag  string
		err  error
	}{
		{"unchanged", "v1.0.0", nil},
		{"empty", "", nil},
		{"error", "", errors.New("github down")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			store := NewMockStore(ctrl)
			releases := NewMockReleaseProvider(ctrl)
			store.EXPECT().
				ClaimDue(gomock.Any(), 10, time.Minute).
				Return([]WatchedRepo{{Repo: "owner/repo", LastSeenTag: "v1.0.0"}}, nil)
			releases.EXPECT().
				GetLatestTag(gomock.Any(), "owner/repo").
				Return(tt.tag, tt.err)
			svc := NewService(store, releases, nil, 10, time.Minute)
			if err := svc.ScanOnce(context.Background()); err != nil {
				t.Fatalf("ScanOnce() error = %v", err)
			}
		})
	}
}
