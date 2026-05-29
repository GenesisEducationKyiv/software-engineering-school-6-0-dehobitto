package service

import (
	"context"
	"errors"
	"testing"

	ghpkg "subber/internal/github"
	"subber/internal/models"
)

// fakeRepo is a test double for SubscriptionRepository.
type fakeRepo struct {
	exists    bool
	existsErr error
	saveErr   error
	saved     models.Subscription
}

func (f *fakeRepo) SubscriptionExists(_ context.Context, _, _ string) (bool, error) {
	return f.exists, f.existsErr
}

func (f *fakeRepo) SaveSubscription(_ context.Context, sub models.Subscription) error {
	f.saved = sub
	return f.saveErr
}

// fakeGitHub is a test double for GitHubClient.
type fakeGitHub struct {
	checkErr error
	tag      string
	tagErr   error
}

func (f *fakeGitHub) CheckIfRepoExists(_ context.Context, _ string) error { return f.checkErr }
func (f *fakeGitHub) GetLatestTag(_ context.Context, _ string) (string, error) {
	return f.tag, f.tagErr
}

func newSvc(repo SubscriptionRepository, gh GitHubClient) (*SubscriptionService, chan models.NotificationJob) {
	jobs := make(chan models.NotificationJob, 1)
	return NewSubscriptionService(repo, "http://localhost", jobs, gh), jobs
}

// — duplicate guard —

func TestSubscribe_RepositoryCheckFails(t *testing.T) {
	// DB errors must surface — silently swallowing them would allow duplicate subscriptions.
	svc, _ := newSvc(&fakeRepo{existsErr: errors.New("db timeout")}, &fakeGitHub{})

	err := svc.Subscribe(context.Background(), "a@b.com", "owner/repo")

	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestSubscribe_AlreadySubscribed(t *testing.T) {
	// Dedup check must short-circuit before hitting GitHub — avoids unnecessary API calls.
	svc, _ := newSvc(&fakeRepo{exists: true}, &fakeGitHub{})

	err := svc.Subscribe(context.Background(), "a@b.com", "owner/repo")

	if !errors.Is(err, ErrAlreadySubscribed) {
		t.Errorf("expected ErrAlreadySubscribed, got %v", err)
	}
}

// — GitHub validation —

func TestSubscribe_RepoNotFound(t *testing.T) {
	// We must not save subscriptions for repos that don't exist on GitHub.
	svc, _ := newSvc(&fakeRepo{}, &fakeGitHub{checkErr: ghpkg.ErrNotFound})

	err := svc.Subscribe(context.Background(), "a@b.com", "owner/repo")

	if !errors.Is(err, ErrRepoNotFound) {
		t.Errorf("expected ErrRepoNotFound, got %v", err)
	}
}

func TestSubscribe_RateLimit(t *testing.T) {
	// Rate limit must be surfaced specifically so callers can retry with backoff.
	svc, _ := newSvc(&fakeRepo{}, &fakeGitHub{checkErr: ghpkg.ErrRateLimit})

	err := svc.Subscribe(context.Background(), "a@b.com", "owner/repo")

	if !errors.Is(err, ErrGitHubRateLimit) {
		t.Errorf("expected ErrGitHubRateLimit, got %v", err)
	}
}

func TestSubscribe_GitHubUnavailable(t *testing.T) {
	// Unknown GitHub errors must map to ErrGitHubUnavailable, not leak internal details.
	svc, _ := newSvc(&fakeRepo{}, &fakeGitHub{checkErr: errors.New("connection refused")})

	err := svc.Subscribe(context.Background(), "a@b.com", "owner/repo")

	if !errors.Is(err, ErrGitHubUnavailable) {
		t.Errorf("expected ErrGitHubUnavailable, got %v", err)
	}
}

// — persistence —

func TestSubscribe_TagFetchFails_ContinuesWithEmptyTag(t *testing.T) {
	// Initial tag fetch is best-effort — a transient GitHub error must not block subscription.
	// The scanner will populate the tag on its first cycle.
	repo := &fakeRepo{}
	svc, _ := newSvc(repo, &fakeGitHub{tagErr: errors.New("timeout")})

	err := svc.Subscribe(context.Background(), "a@b.com", "owner/repo")

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if repo.saved.LastSeenTag != "" {
		t.Errorf("expected empty tag, got %q", repo.saved.LastSeenTag)
	}
}

func TestSubscribe_SaveFails(t *testing.T) {
	// Persistence failure must propagate — no confirmation should be sent for unsaved subscriptions.
	svc, jobs := newSvc(&fakeRepo{saveErr: errors.New("db error")}, &fakeGitHub{tag: "v1.0.0"})

	err := svc.Subscribe(context.Background(), "a@b.com", "owner/repo")

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if len(jobs) != 0 {
		t.Error("confirmation must not be enqueued when save fails")
	}
}

// — success path —

func TestSubscribe_Success_StartsUnconfirmed(t *testing.T) {
	// New subscriptions must require email confirmation — activating without it bypasses the auth flow.
	repo := &fakeRepo{}
	svc, _ := newSvc(repo, &fakeGitHub{tag: "v1.0.0"})

	if err := svc.Subscribe(context.Background(), "a@b.com", "owner/repo"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if repo.saved.Confirmed {
		t.Error("subscription must start unconfirmed")
	}
}

func TestSubscribe_Success_PersistsAllFields(t *testing.T) {
	repo := &fakeRepo{}
	svc, _ := newSvc(repo, &fakeGitHub{tag: "v1.0.0"})

	if err := svc.Subscribe(context.Background(), "a@b.com", "owner/repo"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if repo.saved.Email != "a@b.com" {
		t.Errorf("email: want a@b.com, got %s", repo.saved.Email)
	}
	if repo.saved.Repo != "owner/repo" {
		t.Errorf("repo: want owner/repo, got %s", repo.saved.Repo)
	}
	if repo.saved.LastSeenTag != "v1.0.0" {
		t.Errorf("tag: want v1.0.0, got %s", repo.saved.LastSeenTag)
	}
	if repo.saved.Token == "" {
		t.Error("token must be non-empty")
	}
}

func TestSubscribe_Success_EnqueuesConfirmation(t *testing.T) {
	// Confirmation email must be queued so the user can verify ownership of the email address.
	svc, jobs := newSvc(&fakeRepo{}, &fakeGitHub{tag: "v1.0.0"})

	if err := svc.Subscribe(context.Background(), "a@b.com", "owner/repo"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	select {
	case job := <-jobs:
		if job.Email != "a@b.com" {
			t.Errorf("job email: want a@b.com, got %s", job.Email)
		}
	default:
		t.Error("confirmation job must be enqueued on success")
	}
}
