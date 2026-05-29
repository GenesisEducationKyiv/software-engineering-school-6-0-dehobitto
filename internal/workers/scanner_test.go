package workers

import (
	"context"
	"errors"
	"testing"

	"subber/internal/models"
)

// fakeScanRepo is a test double for ScanRepository.
type fakeScanRepo struct {
	repos       []models.GitHubRelease
	reposErr    error
	subscribers map[string][]string // repo → emails
	subsErr     error
	updateErr   error
	updated     []models.GitHubRelease
}

func (f *fakeScanRepo) GetUniqueSubscriptions(_ context.Context) ([]models.GitHubRelease, error) {
	return f.repos, f.reposErr
}

func (f *fakeScanRepo) GetSubscribers(_ context.Context, repo string) ([]string, error) {
	if f.subsErr != nil {
		return nil, f.subsErr
	}
	return f.subscribers[repo], nil
}

func (f *fakeScanRepo) UpdateTags(_ context.Context, r models.GitHubRelease) error {
	f.updated = append(f.updated, r)
	return f.updateErr
}

// fakeReleaseChecker is a test double for ReleaseChecker.
type fakeReleaseChecker struct {
	tags map[string]string
	errs map[string]error
}

func (f *fakeReleaseChecker) GetLatestTag(_ context.Context, repo string) (string, error) {
	if err, ok := f.errs[repo]; ok {
		return "", err
	}
	return f.tags[repo], nil
}

func newWorker(repo ScanRepository, gh ReleaseChecker) (*ScannerWorker, chan models.NotificationJob) {
	jobs := make(chan models.NotificationJob, 10)
	return &ScannerWorker{repo: repo, jobs: jobs, github: gh}, jobs
}

func TestScan_NoRepos_NoJobsEnqueued(t *testing.T) {
	// Empty subscription list must be a no-op — no updates, no notifications.
	w, jobs := newWorker(&fakeScanRepo{}, &fakeReleaseChecker{})

	if err := w.scan(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(jobs) != 0 {
		t.Errorf("expected no jobs, got %d", len(jobs))
	}
}

func TestScan_TagUnchanged_NoUpdateNoJob(t *testing.T) {
	// Scanner must not notify subscribers when the tag has not changed — prevents spam.
	repo := &fakeScanRepo{
		repos: []models.GitHubRelease{{Repo: "owner/repo", LastSeenTag: "v1.0.0"}},
	}
	gh := &fakeReleaseChecker{tags: map[string]string{"owner/repo": "v1.0.0"}}
	w, jobs := newWorker(repo, gh)

	if err := w.scan(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(repo.updated) != 0 {
		t.Error("UpdateTags must not be called when tag is unchanged")
	}
	if len(jobs) != 0 {
		t.Error("no jobs must be enqueued when tag is unchanged")
	}
}

func TestScan_NewRelease_UpdatesTag(t *testing.T) {
	// A new release tag must be persisted so the next scan cycle doesn't re-notify.
	repo := &fakeScanRepo{
		repos:       []models.GitHubRelease{{Repo: "owner/repo", LastSeenTag: "v1.0.0"}},
		subscribers: map[string][]string{"owner/repo": {"a@b.com"}},
	}
	gh := &fakeReleaseChecker{tags: map[string]string{"owner/repo": "v2.0.0"}}
	w, _ := newWorker(repo, gh)

	if err := w.scan(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(repo.updated) != 1 || repo.updated[0].LastSeenTag != "v2.0.0" {
		t.Errorf("expected UpdateTags with v2.0.0, got %v", repo.updated)
	}
}

func TestScan_NewRelease_NotifiesAllSubscribers(t *testing.T) {
	// Every confirmed subscriber must receive a notification — missing one is a silent data loss.
	repo := &fakeScanRepo{
		repos:       []models.GitHubRelease{{Repo: "owner/repo", LastSeenTag: "v1.0.0"}},
		subscribers: map[string][]string{"owner/repo": {"a@b.com", "c@d.com"}},
	}
	gh := &fakeReleaseChecker{tags: map[string]string{"owner/repo": "v2.0.0"}}
	w, jobs := newWorker(repo, gh)

	if err := w.scan(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(jobs) != 2 {
		t.Errorf("expected 2 notification jobs, got %d", len(jobs))
	}
}

func TestScan_GetLatestTagFails_RepoSkipped(t *testing.T) {
	// A tag fetch error for one repo must not stop the scan — other repos must still be checked.
	repo := &fakeScanRepo{
		repos: []models.GitHubRelease{
			{Repo: "owner/bad", LastSeenTag: "v1.0.0"},
			{Repo: "owner/good", LastSeenTag: "v1.0.0"},
		},
		subscribers: map[string][]string{"owner/good": {"a@b.com"}},
	}
	gh := &fakeReleaseChecker{
		tags: map[string]string{"owner/good": "v2.0.0"},
		errs: map[string]error{"owner/bad": errors.New("timeout")},
	}
	w, jobs := newWorker(repo, gh)

	if err := w.scan(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(jobs) != 1 {
		t.Errorf("expected 1 job for healthy repo, got %d", len(jobs))
	}
}

func TestScan_UpdateTagsFails_NoJobEnqueued(t *testing.T) {
	// If the new tag cannot be persisted, we must not notify — the next cycle would notify again,
	// causing duplicate notifications.
	repo := &fakeScanRepo{
		repos:     []models.GitHubRelease{{Repo: "owner/repo", LastSeenTag: "v1.0.0"}},
		updateErr: errors.New("db error"),
	}
	gh := &fakeReleaseChecker{tags: map[string]string{"owner/repo": "v2.0.0"}}
	w, jobs := newWorker(repo, gh)

	if err := w.scan(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(jobs) != 0 {
		t.Error("job must not be enqueued when UpdateTags fails")
	}
}

func TestScan_GetSubscribersFails_NoJobEnqueued(t *testing.T) {
	// Subscriber fetch failure must not panic and must not send partial notifications.
	repo := &fakeScanRepo{
		repos:       []models.GitHubRelease{{Repo: "owner/repo", LastSeenTag: "v1.0.0"}},
		subscribers: map[string][]string{},
		subsErr:     errors.New("db error"),
	}
	gh := &fakeReleaseChecker{tags: map[string]string{"owner/repo": "v2.0.0"}}
	w, jobs := newWorker(repo, gh)

	if err := w.scan(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(jobs) != 0 {
		t.Error("no jobs must be enqueued when subscriber fetch fails")
	}
}
