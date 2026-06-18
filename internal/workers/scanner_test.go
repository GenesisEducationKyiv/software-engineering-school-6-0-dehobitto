package workers

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"

	"subber/internal/logger"
	"subber/internal/metrics"
	"subber/internal/models"
)

type mockScanRepository struct {
	mock.Mock
}

func (m *mockScanRepository) GetUniqueSubscriptions(ctx context.Context) ([]models.GitHubRelease, error) {
	args := m.Called(ctx)
	return args.Get(0).([]models.GitHubRelease), args.Error(1)
}

func (m *mockScanRepository) GetSubscribers(ctx context.Context, repo string) ([]string, error) {
	args := m.Called(ctx, repo)
	return args.Get(0).([]string), args.Error(1)
}

func (m *mockScanRepository) UpdateTags(ctx context.Context, r models.GitHubRelease) error {
	return m.Called(ctx, r).Error(0)
}

type mockReleaseChecker struct {
	mock.Mock
}

func (m *mockReleaseChecker) GetLatestTag(ctx context.Context, repo string) (string, error) {
	args := m.Called(ctx, repo)
	return args.String(0), args.Error(1)
}

func newWorker(repo ScanRepository, gh ReleaseChecker) (*ScannerWorker, chan models.NotificationJob) {
	jobs := make(chan models.NotificationJob, 10)
	return NewScannerWorker(repo, jobs, gh, logger.NewNoop(), metrics.NewNoop()), jobs
}

func expectUniqueSubscriptions(repo *mockScanRepository, releases []models.GitHubRelease, err error) {
	repo.On("GetUniqueSubscriptions", mock.Anything).Return(releases, err).Once()
}

func expectLatestTag(gh *mockReleaseChecker, repo string, tag string, err error) {
	gh.On("GetLatestTag", mock.Anything, repo).Return(tag, err).Once()
}

func expectUpdate(repo *mockScanRepository, release models.GitHubRelease, err error) {
	repo.On("UpdateTags", mock.Anything, mock.MatchedBy(func(got models.GitHubRelease) bool {
		return got.Repo == release.Repo && got.LastSeenTag == release.LastSeenTag
	})).Return(err).Once()
}

func expectSubscribers(repo *mockScanRepository, name string, emails []string, err error) {
	repo.On("GetSubscribers", mock.Anything, name).Return(emails, err).Once()
}

func TestScan_NoRepos_NoJobsEnqueued(t *testing.T) {
	// Empty subscription list must be a no-op — no updates, no notifications.
	repo := new(mockScanRepository)
	expectUniqueSubscriptions(repo, nil, nil)
	w, jobs := newWorker(repo, new(mockReleaseChecker))

	if err := w.scan(context.Background(), "test-scan-cycle"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	repo.AssertExpectations(t)

	if len(jobs) != 0 {
		t.Errorf("expected no jobs, got %d", len(jobs))
	}
}

func TestScan_TagUnchanged_NoUpdateNoJob(t *testing.T) {
	// Scanner must not notify subscribers when the tag has not changed — prevents spam.
	releases := []models.GitHubRelease{{Repo: "owner/repo", LastSeenTag: "v1.0.0"}}
	repo := new(mockScanRepository)
	gh := new(mockReleaseChecker)
	expectUniqueSubscriptions(repo, releases, nil)
	expectLatestTag(gh, "owner/repo", "v1.0.0", nil)
	w, jobs := newWorker(repo, gh)

	if err := w.scan(context.Background(), "test-scan-cycle"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	repo.AssertExpectations(t)
	gh.AssertExpectations(t)

	repo.AssertNotCalled(t, "UpdateTags", mock.Anything, mock.Anything)
	if len(jobs) != 0 {
		t.Error("no jobs must be enqueued when tag is unchanged")
	}
}

func TestScan_NewRelease_UpdatesTag(t *testing.T) {
	// A new release tag must be persisted so the next scan cycle doesn't re-notify.
	releases := []models.GitHubRelease{{Repo: "owner/repo", LastSeenTag: "v1.0.0"}}
	repo := new(mockScanRepository)
	gh := new(mockReleaseChecker)
	expectUniqueSubscriptions(repo, releases, nil)
	expectLatestTag(gh, "owner/repo", "v2.0.0", nil)
	expectUpdate(repo, models.GitHubRelease{Repo: "owner/repo", LastSeenTag: "v2.0.0"}, nil)
	expectSubscribers(repo, "owner/repo", []string{"a@b.com"}, nil)
	w, _ := newWorker(repo, gh)

	if err := w.scan(context.Background(), "test-scan-cycle"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	repo.AssertExpectations(t)
	gh.AssertExpectations(t)
}

func TestScan_NewRelease_NotifiesAllSubscribers(t *testing.T) {
	// Every confirmed subscriber must receive a notification — missing one is a silent data loss.
	releases := []models.GitHubRelease{{Repo: "owner/repo", LastSeenTag: "v1.0.0"}}
	repo := new(mockScanRepository)
	gh := new(mockReleaseChecker)
	expectUniqueSubscriptions(repo, releases, nil)
	expectLatestTag(gh, "owner/repo", "v2.0.0", nil)
	expectUpdate(repo, models.GitHubRelease{Repo: "owner/repo", LastSeenTag: "v2.0.0"}, nil)
	expectSubscribers(repo, "owner/repo", []string{"a@b.com", "c@d.com"}, nil)
	w, jobs := newWorker(repo, gh)

	if err := w.scan(context.Background(), "test-scan-cycle"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	repo.AssertExpectations(t)
	gh.AssertExpectations(t)

	if len(jobs) != 2 {
		t.Errorf("expected 2 notification jobs, got %d", len(jobs))
	}
	job := <-jobs
	if job.ScanCycleID != "test-scan-cycle" {
		t.Errorf("job scan cycle id: want test-scan-cycle, got %q", job.ScanCycleID)
	}
}

func TestScan_GetLatestTagFails_RepoSkipped(t *testing.T) {
	// A tag fetch error for one repo must not stop the scan — other repos must still be checked.
	releases := []models.GitHubRelease{
		{Repo: "owner/bad", LastSeenTag: "v1.0.0"},
		{Repo: "owner/good", LastSeenTag: "v1.0.0"},
	}
	repo := new(mockScanRepository)
	gh := new(mockReleaseChecker)
	expectUniqueSubscriptions(repo, releases, nil)
	expectLatestTag(gh, "owner/bad", "", errors.New("timeout"))
	expectLatestTag(gh, "owner/good", "v2.0.0", nil)
	expectUpdate(repo, models.GitHubRelease{Repo: "owner/good", LastSeenTag: "v2.0.0"}, nil)
	expectSubscribers(repo, "owner/good", []string{"a@b.com"}, nil)
	w, jobs := newWorker(repo, gh)

	if err := w.scan(context.Background(), "test-scan-cycle"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	repo.AssertExpectations(t)
	gh.AssertExpectations(t)

	if len(jobs) != 1 {
		t.Errorf("expected 1 job for healthy repo, got %d", len(jobs))
	}
}

func TestScan_UpdateTagsFails_NoJobEnqueued(t *testing.T) {
	// If the new tag cannot be persisted, we must not notify — the next cycle would notify again,
	// causing duplicate notifications.
	releases := []models.GitHubRelease{{Repo: "owner/repo", LastSeenTag: "v1.0.0"}}
	repo := new(mockScanRepository)
	gh := new(mockReleaseChecker)
	expectUniqueSubscriptions(repo, releases, nil)
	expectLatestTag(gh, "owner/repo", "v2.0.0", nil)
	expectUpdate(repo, models.GitHubRelease{Repo: "owner/repo", LastSeenTag: "v2.0.0"}, errors.New("db error"))
	w, jobs := newWorker(repo, gh)

	if err := w.scan(context.Background(), "test-scan-cycle"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	repo.AssertExpectations(t)
	gh.AssertExpectations(t)
	repo.AssertNotCalled(t, "GetSubscribers", mock.Anything, "owner/repo")

	if len(jobs) != 0 {
		t.Error("job must not be enqueued when UpdateTags fails")
	}
}

func TestScan_GetSubscribersFails_NoJobEnqueued(t *testing.T) {
	// Subscriber fetch failure must not panic and must not send partial notifications.
	releases := []models.GitHubRelease{{Repo: "owner/repo", LastSeenTag: "v1.0.0"}}
	repo := new(mockScanRepository)
	gh := new(mockReleaseChecker)
	expectUniqueSubscriptions(repo, releases, nil)
	expectLatestTag(gh, "owner/repo", "v2.0.0", nil)
	expectUpdate(repo, models.GitHubRelease{Repo: "owner/repo", LastSeenTag: "v2.0.0"}, nil)
	expectSubscribers(repo, "owner/repo", nil, errors.New("db error"))
	w, jobs := newWorker(repo, gh)

	if err := w.scan(context.Background(), "test-scan-cycle"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	repo.AssertExpectations(t)
	gh.AssertExpectations(t)

	if len(jobs) != 0 {
		t.Error("no jobs must be enqueued when subscriber fetch fails")
	}
}
