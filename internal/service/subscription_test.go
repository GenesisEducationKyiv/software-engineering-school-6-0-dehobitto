package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"subber/internal/config"
	"subber/internal/infra/cache"
	"subber/internal/models"
	"subber/internal/workers"
)

// fakeRepo records the last subscription passed to SaveSubscription.
type fakeRepo struct {
	saved models.Subscription
}

func (f *fakeRepo) SubscriptionExists(_ context.Context, _, _ string) (bool, error) {
	return false, nil
}

func (f *fakeRepo) SaveSubscription(_ context.Context, sub models.Subscription) error {
	f.saved = sub
	return nil
}

// fakeGitHub returns 200 for repo check and an empty tag.
type fakeGitHub struct{}

func (fakeGitHub) CheckIfRepoExists(_ context.Context, _, _ string) (*http.Response, error) {
	rec := httptest.NewRecorder()
	rec.WriteHeader(http.StatusOK)
	return rec.Result(), nil
}

func (fakeGitHub) GetLatestTag(_ context.Context, _, _ string, _ cache.Cache) (string, error) {
	return "", nil
}

// fixedUUIDGen always returns the same token so tests can assert on it.
type fixedUUIDGen struct{ token string }

func (g fixedUUIDGen) New() string { return g.token }

func newTestService(repo SubscriptionRepository, gh gitHubClient, gen UUIDGenerator) *SubscriptionService {
	jobs := make(chan workers.NotificationJob, 1)
	cfg := &config.Config{BaseURL: "http://localhost"}
	return NewSubscriptionService(repo, cfg, jobs, nil, gh, gen)
}

func TestSubscribe_TokenComesfromUUIDGenerator(t *testing.T) {
	const wantToken = "fixed-test-token"

	repo := &fakeRepo{}
	svc := newTestService(repo, fakeGitHub{}, fixedUUIDGen{token: wantToken})

	err := svc.Subscribe(context.Background(), "user@example.com", "owner/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if repo.saved.Token != wantToken {
		t.Errorf("token = %q, want %q", repo.saved.Token, wantToken)
	}
}
