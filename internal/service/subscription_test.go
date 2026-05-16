package service

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"subber/internal/config"
	"subber/internal/infra/cache"
	"subber/internal/models"
	"subber/internal/workers"
)

type fakeRepo struct {
	saved  models.Subscription
	exists bool
}

func (f *fakeRepo) SubscriptionExists(_ context.Context, _, _ string) (bool, error) {
	return f.exists, nil
}

func (f *fakeRepo) SaveSubscription(_ context.Context, sub models.Subscription) error {
	f.saved = sub
	return nil
}

// fakeGitHub returns configurable status for repo check.
type fakeGitHub struct{ status int }

func (f fakeGitHub) CheckIfRepoExists(_ context.Context, _, _ string) (*http.Response, error) {
	rec := httptest.NewRecorder()
	rec.WriteHeader(f.status)
	return rec.Result(), nil
}

func (fakeGitHub) GetLatestTag(_ context.Context, _, _ string, _ cache.Cache) (string, error) {
	return "", nil
}

func okGitHub() fakeGitHub        { return fakeGitHub{status: http.StatusOK} }
func noRepoGitHub() fakeGitHub    { return fakeGitHub{status: http.StatusNotFound} }
func rateLimitGitHub() fakeGitHub { return fakeGitHub{status: http.StatusTooManyRequests} }

// errorGitHub simulates a network failure on repo check.
type errorGitHub struct{}

func (errorGitHub) CheckIfRepoExists(_ context.Context, _, _ string) (*http.Response, error) {
	return nil, errors.New("connection refused")
}

func (errorGitHub) GetLatestTag(_ context.Context, _, _ string, _ cache.Cache) (string, error) {
	return "", nil
}

type fixedUUIDGen struct{ token string }

func (g fixedUUIDGen) New() string { return g.token }

func TestSubscribe_EnqueuesConfirmationEmail(t *testing.T) {
	jobs := make(chan workers.NotificationJob, 1)
	cfg := &config.Config{BaseURL: "http://localhost"}
	svc := NewSubscriptionService(&fakeRepo{}, cfg, jobs, nil, okGitHub(), fixedUUIDGen{token: "tok"})

	err := svc.Subscribe(context.Background(), "user@example.com", "owner/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	select {
	case job := <-jobs:
		if job.Email != "user@example.com" {
			t.Errorf("job.Email = %q, want user@example.com", job.Email)
		}
		if !strings.Contains(job.Message, "tok") {
			t.Errorf("confirmation message missing token: %q", job.Message)
		}
	default:
		t.Fatal("no job enqueued")
	}
}

func TestSubscribe_ReturnsErrGitHubUnavailable_WhenNetworkFails(t *testing.T) {
	svc := newTestService(&fakeRepo{}, errorGitHub{}, fixedUUIDGen{token: "x"})

	err := svc.Subscribe(context.Background(), "user@example.com", "owner/repo")

	if !errors.Is(err, ErrGitHubUnavailable) {
		t.Errorf("err = %v, want ErrGitHubUnavailable", err)
	}
}

func TestSubscribe_ReturnsErrGitHubRateLimit_WhenGitHubReturns429(t *testing.T) {
	svc := newTestService(&fakeRepo{}, rateLimitGitHub(), fixedUUIDGen{token: "x"})

	err := svc.Subscribe(context.Background(), "user@example.com", "owner/repo")

	if !errors.Is(err, ErrGitHubRateLimit) {
		t.Errorf("err = %v, want ErrGitHubRateLimit", err)
	}
}

func TestSubscribe_ReturnsErrRepoNotFound_WhenGitHubReturns404(t *testing.T) {
	svc := newTestService(&fakeRepo{}, noRepoGitHub(), fixedUUIDGen{token: "x"})

	err := svc.Subscribe(context.Background(), "user@example.com", "owner/repo")

	if !errors.Is(err, ErrRepoNotFound) {
		t.Errorf("err = %v, want ErrRepoNotFound", err)
	}
}

func newTestService(repo SubscriptionRepository, gh gitHubClient, gen UUIDGenerator) *SubscriptionService {
	jobs := make(chan workers.NotificationJob, 1)
	cfg := &config.Config{BaseURL: "http://localhost"}
	return NewSubscriptionService(repo, cfg, jobs, nil, gh, gen)
}

func TestSubscribe_ReturnsErrAlreadySubscribed_WhenDuplicate(t *testing.T) {
	repo := &fakeRepo{exists: true}
	svc := newTestService(repo, okGitHub(), fixedUUIDGen{token: "x"})

	err := svc.Subscribe(context.Background(), "user@example.com", "owner/repo")

	if !errors.Is(err, ErrAlreadySubscribed) {
		t.Errorf("err = %v, want ErrAlreadySubscribed", err)
	}
}

func TestSubscribe_TokenComesfromUUIDGenerator(t *testing.T) {
	const wantToken = "fixed-test-token"

	repo := &fakeRepo{}
	svc := newTestService(repo, okGitHub(), fixedUUIDGen{token: wantToken})

	err := svc.Subscribe(context.Background(), "user@example.com", "owner/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if repo.saved.Token != wantToken {
		t.Errorf("token = %q, want %q", repo.saved.Token, wantToken)
	}
}
