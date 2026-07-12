package github

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
)

type MockCache struct {
	ctrl     *gomock.Controller
	recorder *MockCacheMockRecorder
}

type MockCacheMockRecorder struct {
	mock *MockCache
}

func NewMockCache(ctrl *gomock.Controller) *MockCache {
	mock := &MockCache{ctrl: ctrl}
	mock.recorder = &MockCacheMockRecorder{mock}
	return mock
}

func (m *MockCache) EXPECT() *MockCacheMockRecorder {
	return m.recorder
}

func (m *MockCache) Get(ctx context.Context, key string) (string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Get", ctx, key)
	ret0, _ := ret[0].(string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

func (mr *MockCacheMockRecorder) Get(ctx, key interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Get", reflect.TypeOf((*MockCache)(nil).Get), ctx, key)
}

func (m *MockCache) Set(ctx context.Context, key, value string, ttl time.Duration) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Set", ctx, key, value, ttl)
	ret0, _ := ret[0].(error)
	return ret0
}

func (mr *MockCacheMockRecorder) Set(ctx, key, value, ttl interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Set", reflect.TypeOf((*MockCache)(nil).Set), ctx, key, value, ttl)
}

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

func TestCachedReleaseProvider_CacheHitAvoidsGitHub(t *testing.T) {
	ctrl := gomock.NewController(t)
	cache := NewMockCache(ctrl)
	next := NewMockReleaseProvider(ctrl)
	cache.EXPECT().Get(gomock.Any(), "github:latest_tag:owner/repo").Return("v1.0.0", nil)
	provider := NewCachedReleaseProvider(cache, next, time.Minute, nil)

	tag, err := provider.GetLatestTag(context.Background(), "owner/repo")
	if err != nil {
		t.Fatalf("GetLatestTag() error = %v", err)
	}
	if tag != "v1.0.0" {
		t.Fatalf("tag = %q, want v1.0.0", tag)
	}
}

func TestCachedReleaseProvider_CacheMissCallsGitHubAndStores(t *testing.T) {
	ctrl := gomock.NewController(t)
	cache := NewMockCache(ctrl)
	next := NewMockReleaseProvider(ctrl)
	gomock.InOrder(
		cache.EXPECT().Get(gomock.Any(), "github:latest_tag:owner/repo").Return("", nil),
		next.EXPECT().GetLatestTag(gomock.Any(), "owner/repo").Return("v2.0.0", nil),
		cache.EXPECT().Set(gomock.Any(), "github:latest_tag:owner/repo", "v2.0.0", time.Minute).Return(nil),
	)
	provider := NewCachedReleaseProvider(cache, next, time.Minute, nil)

	tag, err := provider.GetLatestTag(context.Background(), "owner/repo")
	if err != nil {
		t.Fatalf("GetLatestTag() error = %v", err)
	}
	if tag != "v2.0.0" {
		t.Fatalf("tag = %q, want v2.0.0", tag)
	}
}

func TestCachedReleaseProvider_DoesNotCacheErrors(t *testing.T) {
	ctrl := gomock.NewController(t)
	cache := NewMockCache(ctrl)
	next := NewMockReleaseProvider(ctrl)
	githubErr := errors.New("github down")
	gomock.InOrder(
		cache.EXPECT().Get(gomock.Any(), "github:latest_tag:owner/repo").Return("", nil),
		next.EXPECT().GetLatestTag(gomock.Any(), "owner/repo").Return("", githubErr),
	)
	provider := NewCachedReleaseProvider(cache, next, time.Minute, nil)

	if _, err := provider.GetLatestTag(context.Background(), "owner/repo"); err == nil {
		t.Fatal("expected error")
	}
}
