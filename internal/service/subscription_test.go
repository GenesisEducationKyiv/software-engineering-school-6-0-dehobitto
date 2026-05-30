package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	gh "subber/internal/github"
	"subber/internal/models"
)

// --- test doubles ---

type fakeRepo struct {
	saved     models.Subscription
	exists    bool
	existsErr error
	saveErr   error
}

func (f *fakeRepo) SubscriptionExists(_ context.Context, _, _ string) (bool, error) {
	return f.exists, f.existsErr
}
func (f *fakeRepo) SaveSubscription(_ context.Context, sub models.Subscription) error {
	f.saved = sub
	return f.saveErr
}

type fakeGitHub struct{ checkErr error }

func (f fakeGitHub) CheckIfRepoExists(_ context.Context, _ string) error    { return f.checkErr }
func (fakeGitHub) GetLatestTag(_ context.Context, _ string) (string, error) { return "", nil }

type fixedUUIDGen struct{ token string }

func (g fixedUUIDGen) New() string { return g.token }

func newTestService(repo SubscriptionRepository, g GitHubClient) *SubscriptionService {
	jobs := make(chan models.NotificationJob, 1)
	return NewSubscriptionService(repo, "http://localhost", jobs, g, fixedUUIDGen{token: "test-token"})
}

// --- tests ---

func TestSubscribe_HappyPath(t *testing.T) {
	const token = "fixed-token"
	repo := &fakeRepo{}
	jobs := make(chan models.NotificationJob, 1)
	svc := NewSubscriptionService(repo, "http://localhost", jobs, fakeGitHub{}, fixedUUIDGen{token: token})

	if err := svc.Subscribe(context.Background(), "user@example.com", "owner/repo"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// token from UUID generator must be persisted
	if repo.saved.Token != token {
		t.Errorf("saved token = %q, want %q", repo.saved.Token, token)
	}

	// confirmation job must be enqueued with correct email and token in message
	select {
	case job := <-jobs:
		if job.Email != "user@example.com" {
			t.Errorf("job.Email = %q, want user@example.com", job.Email)
		}
		if !strings.Contains(job.Message, token) {
			t.Errorf("confirmation message missing token: %q", job.Message)
		}
	default:
		t.Fatal("no confirmation job enqueued")
	}
}

func TestSubscribe_ReturnsErrAlreadySubscribed_WhenDuplicate(t *testing.T) {
	svc := newTestService(&fakeRepo{exists: true}, fakeGitHub{})

	err := svc.Subscribe(context.Background(), "user@example.com", "owner/repo")

	if !errors.Is(err, ErrAlreadySubscribed) {
		t.Errorf("err = %v, want ErrAlreadySubscribed", err)
	}
}

func TestSubscribe_RepoErrors(t *testing.T) {
	tests := []struct {
		name      string
		existsErr error
		saveErr   error
	}{
		{"exists check fails", errors.New("db timeout"), nil},
		{"save fails", nil, errors.New("db timeout")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeRepo{existsErr: tt.existsErr, saveErr: tt.saveErr}
			svc := newTestService(repo, fakeGitHub{})
			err := svc.Subscribe(context.Background(), "user@example.com", "owner/repo")
			if err == nil {
				t.Error("want error, got nil")
			}
		})
	}
}

func TestSubscribe_GitHubErrors(t *testing.T) {
	tests := []struct {
		name    string
		ghErr   error
		wantErr error
	}{
		{"network failure → ErrGitHubUnavailable", errors.New("connection refused"), ErrGitHubUnavailable},
		{"rate limit → ErrGitHubRateLimit", gh.ErrRateLimit, ErrGitHubRateLimit},
		{"repo not found → ErrRepoNotFound", gh.ErrNotFound, ErrRepoNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := newTestService(&fakeRepo{}, fakeGitHub{checkErr: tt.ghErr})
			err := svc.Subscribe(context.Background(), "user@example.com", "owner/repo")
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("err = %v, want %v", err, tt.wantErr)
			}
		})
	}
}
