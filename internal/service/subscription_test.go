package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"

	ghpkg "subber/internal/github"
	"subber/internal/models"
)

type mockSubscriptionRepository struct {
	mock.Mock
}

func (m *mockSubscriptionRepository) SubscriptionExists(ctx context.Context, email, repo string) (bool, error) {
	args := m.Called(ctx, email, repo)
	return args.Bool(0), args.Error(1)
}

func (m *mockSubscriptionRepository) SaveSubscription(ctx context.Context, sub models.Subscription) error {
	return m.Called(ctx, sub).Error(0)
}

type mockGitHubClient struct {
	mock.Mock
}

func (m *mockGitHubClient) CheckIfRepoExists(ctx context.Context, repo string) error {
	return m.Called(ctx, repo).Error(0)
}

func (m *mockGitHubClient) GetLatestTag(ctx context.Context, repo string) (string, error) {
	args := m.Called(ctx, repo)
	return args.String(0), args.Error(1)
}

func newSvc(repo SubscriptionRepository, gh GitHubClient) (*SubscriptionService, chan models.NotificationJob) {
	jobs := make(chan models.NotificationJob, 1)
	return NewSubscriptionService(repo, "http://localhost", jobs, gh, RealUUIDGenerator), jobs
}

func expectSubscriptionExists(repo *mockSubscriptionRepository, exists bool, err error) {
	repo.On("SubscriptionExists", mock.Anything, "a@b.com", "owner/repo").Return(exists, err).Once()
}

func expectRepoValid(gh *mockGitHubClient) {
	gh.On("CheckIfRepoExists", mock.Anything, "owner/repo").Return(nil).Once()
}

func expectSave(repo *mockSubscriptionRepository, saved *models.Subscription, err error) {
	repo.On("SaveSubscription", mock.Anything, mock.MatchedBy(func(sub models.Subscription) bool {
		*saved = sub
		return true
	})).Return(err).Once()
}

// — duplicate guard —

func TestSubscribe_RepositoryCheckFails(t *testing.T) {
	// DB errors must surface — silently swallowing them would allow duplicate subscriptions.
	repo := new(mockSubscriptionRepository)
	expectSubscriptionExists(repo, false, errors.New("db timeout"))
	svc, _ := newSvc(repo, new(mockGitHubClient))

	err := svc.Subscribe(context.Background(), "a@b.com", "owner/repo")
	repo.AssertExpectations(t)

	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestSubscribe_AlreadySubscribed(t *testing.T) {
	// Dedup check must short-circuit before hitting GitHub — avoids unnecessary API calls.
	repo := new(mockSubscriptionRepository)
	expectSubscriptionExists(repo, true, nil)
	svc, _ := newSvc(repo, new(mockGitHubClient))

	err := svc.Subscribe(context.Background(), "a@b.com", "owner/repo")
	repo.AssertExpectations(t)

	if !errors.Is(err, ErrAlreadySubscribed) {
		t.Errorf("expected ErrAlreadySubscribed, got %v", err)
	}
}

// — GitHub validation —

func TestSubscribe_RepoNotFound(t *testing.T) {
	// We must not save subscriptions for repos that don't exist on GitHub.
	repo := new(mockSubscriptionRepository)
	gh := new(mockGitHubClient)
	expectSubscriptionExists(repo, false, nil)
	gh.On("CheckIfRepoExists", mock.Anything, "owner/repo").Return(ghpkg.ErrNotFound).Once()
	svc, _ := newSvc(repo, gh)

	err := svc.Subscribe(context.Background(), "a@b.com", "owner/repo")
	repo.AssertExpectations(t)
	gh.AssertExpectations(t)

	if !errors.Is(err, ErrRepoNotFound) {
		t.Errorf("expected ErrRepoNotFound, got %v", err)
	}
}

func TestSubscribe_RateLimit(t *testing.T) {
	// Rate limit must be surfaced specifically so callers can retry with backoff.
	repo := new(mockSubscriptionRepository)
	gh := new(mockGitHubClient)
	expectSubscriptionExists(repo, false, nil)
	gh.On("CheckIfRepoExists", mock.Anything, "owner/repo").Return(ghpkg.ErrRateLimit).Once()
	svc, _ := newSvc(repo, gh)

	err := svc.Subscribe(context.Background(), "a@b.com", "owner/repo")
	repo.AssertExpectations(t)
	gh.AssertExpectations(t)

	if !errors.Is(err, ErrGitHubRateLimit) {
		t.Errorf("expected ErrGitHubRateLimit, got %v", err)
	}
}

func TestSubscribe_GitHubUnavailable(t *testing.T) {
	// Unknown GitHub errors must map to ErrGitHubUnavailable, not leak internal details.
	repo := new(mockSubscriptionRepository)
	gh := new(mockGitHubClient)
	expectSubscriptionExists(repo, false, nil)
	gh.On("CheckIfRepoExists", mock.Anything, "owner/repo").Return(errors.New("connection refused")).Once()
	svc, _ := newSvc(repo, gh)

	err := svc.Subscribe(context.Background(), "a@b.com", "owner/repo")
	repo.AssertExpectations(t)
	gh.AssertExpectations(t)

	if !errors.Is(err, ErrGitHubUnavailable) {
		t.Errorf("expected ErrGitHubUnavailable, got %v", err)
	}
}

// — persistence —

func TestSubscribe_TagFetchFails_ContinuesWithEmptyTag(t *testing.T) {
	// Initial tag fetch is best-effort — a transient GitHub error must not block subscription.
	// The scanner will populate the tag on its first cycle.
	repo := new(mockSubscriptionRepository)
	gh := new(mockGitHubClient)
	var saved models.Subscription
	expectSubscriptionExists(repo, false, nil)
	expectRepoValid(gh)
	gh.On("GetLatestTag", mock.Anything, "owner/repo").Return("", errors.New("timeout")).Once()
	expectSave(repo, &saved, nil)
	svc, _ := newSvc(repo, gh)

	err := svc.Subscribe(context.Background(), "a@b.com", "owner/repo")
	repo.AssertExpectations(t)
	gh.AssertExpectations(t)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if saved.LastSeenTag != "" {
		t.Errorf("expected empty tag, got %q", saved.LastSeenTag)
	}
}

func TestSubscribe_SaveFails(t *testing.T) {
	// Persistence failure must propagate — no confirmation should be sent for unsaved subscriptions.
	repo := new(mockSubscriptionRepository)
	gh := new(mockGitHubClient)
	var saved models.Subscription
	expectSubscriptionExists(repo, false, nil)
	expectRepoValid(gh)
	gh.On("GetLatestTag", mock.Anything, "owner/repo").Return("v1.0.0", nil).Once()
	expectSave(repo, &saved, errors.New("db error"))
	svc, jobs := newSvc(repo, gh)

	err := svc.Subscribe(context.Background(), "a@b.com", "owner/repo")
	repo.AssertExpectations(t)
	gh.AssertExpectations(t)

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
	repo := new(mockSubscriptionRepository)
	gh := new(mockGitHubClient)
	var saved models.Subscription
	expectSubscriptionExists(repo, false, nil)
	expectRepoValid(gh)
	gh.On("GetLatestTag", mock.Anything, "owner/repo").Return("v1.0.0", nil).Once()
	expectSave(repo, &saved, nil)
	svc, _ := newSvc(repo, gh)

	if err := svc.Subscribe(context.Background(), "a@b.com", "owner/repo"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	repo.AssertExpectations(t)
	gh.AssertExpectations(t)

	if saved.Confirmed {
		t.Error("subscription must start unconfirmed")
	}
}

func TestSubscribe_Success_PersistsAllFields(t *testing.T) {
	repo := new(mockSubscriptionRepository)
	gh := new(mockGitHubClient)
	var saved models.Subscription
	expectSubscriptionExists(repo, false, nil)
	expectRepoValid(gh)
	gh.On("GetLatestTag", mock.Anything, "owner/repo").Return("v1.0.0", nil).Once()
	expectSave(repo, &saved, nil)
	svc, _ := newSvc(repo, gh)

	if err := svc.Subscribe(context.Background(), "a@b.com", "owner/repo"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	repo.AssertExpectations(t)
	gh.AssertExpectations(t)

	if saved.Email != "a@b.com" {
		t.Errorf("email: want a@b.com, got %s", saved.Email)
	}
	if saved.Repo != "owner/repo" {
		t.Errorf("repo: want owner/repo, got %s", saved.Repo)
	}
	if saved.LastSeenTag != "v1.0.0" {
		t.Errorf("tag: want v1.0.0, got %s", saved.LastSeenTag)
	}
	if saved.Token == "" {
		t.Error("token must be non-empty")
	}
}

func TestSubscribe_Success_EnqueuesConfirmation(t *testing.T) {
	// Confirmation email must be queued so the user can verify ownership of the email address.
	repo := new(mockSubscriptionRepository)
	gh := new(mockGitHubClient)
	var saved models.Subscription
	expectSubscriptionExists(repo, false, nil)
	expectRepoValid(gh)
	gh.On("GetLatestTag", mock.Anything, "owner/repo").Return("v1.0.0", nil).Once()
	expectSave(repo, &saved, nil)
	svc, jobs := newSvc(repo, gh)

	if err := svc.Subscribe(context.Background(), "a@b.com", "owner/repo"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	repo.AssertExpectations(t)
	gh.AssertExpectations(t)

	select {
	case job := <-jobs:
		if job.Email != "a@b.com" {
			t.Errorf("job email: want a@b.com, got %s", job.Email)
		}
	default:
		t.Error("confirmation job must be enqueued on success")
	}
}
