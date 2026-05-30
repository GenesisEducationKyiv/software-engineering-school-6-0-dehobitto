package workers

import (
	"context"
	"errors"
	"strings"
	"testing"

	"subber/internal/models"
)

// --- test doubles ---

type fakeTagFetcher struct {
	tag string
	err error
}

func (f fakeTagFetcher) GetLatestTag(_ context.Context, _ string) (string, error) {
	return f.tag, f.err
}

type fakeScanRepo struct {
	subs        []models.GitHubRelease
	subscribers []string
	updatedRepo string
	updateErr   error
}

func (r *fakeScanRepo) GetUniqueSubscriptions(_ context.Context) ([]models.GitHubRelease, error) {
	return r.subs, nil
}
func (r *fakeScanRepo) GetSubscribers(_ context.Context, _ string) ([]string, error) {
	return r.subscribers, nil
}
func (r *fakeScanRepo) UpdateTags(_ context.Context, repo models.GitHubRelease) error {
	r.updatedRepo = repo.Repo
	return r.updateErr
}

func newTestScanner(repo *fakeScanRepo, gh ReleaseChecker) (*ScannerWorker, chan models.NotificationJob) {
	jobs := make(chan models.NotificationJob, 10)
	return NewScannerWorker(repo, jobs, gh), jobs
}

// --- tests ---

func TestScanner_DoesNotEnqueue(t *testing.T) {
	subs := []models.GitHubRelease{{Repo: "owner/repo", LastSeenTag: "v1.0.0"}}

	tests := []struct {
		name string
		gh   ReleaseChecker
	}{
		{"github error", fakeTagFetcher{err: errors.New("timeout")}},
		{"empty tag returned", fakeTagFetcher{tag: ""}},
		{"tag unchanged", fakeTagFetcher{tag: "v1.0.0"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scanner, jobs := newTestScanner(&fakeScanRepo{subs: subs}, tt.gh)
			if err := scanner.scan(context.Background()); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(jobs) != 0 {
				t.Errorf("jobs = %d, want 0", len(jobs))
			}
		})
	}
}

func TestScanner_EnqueuesJob_WhenNewTagDetected(t *testing.T) {
	repo := &fakeScanRepo{
		subs:        []models.GitHubRelease{{Repo: "owner/repo", LastSeenTag: "v1.0.0"}},
		subscribers: []string{"user@example.com"},
	}
	scanner, jobs := newTestScanner(repo, fakeTagFetcher{tag: "v2.0.0"})

	if err := scanner.scan(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("jobs = %d, want 1", len(jobs))
	}

	job := <-jobs
	if job.Email != "user@example.com" {
		t.Errorf("job.Email = %q, want user@example.com", job.Email)
	}
	if !strings.Contains(job.Message, "v2.0.0") {
		t.Errorf("message missing new tag: %q", job.Message)
	}
	if !strings.Contains(job.Message, "owner/repo") {
		t.Errorf("message missing repo: %q", job.Message)
	}
}

func TestScanner_SkipsNotification_WhenUpdateTagsFails(t *testing.T) {
	// Must not notify if the DB update failed — otherwise the old tag stays in DB
	// and the next scan cycle sends a duplicate notification.
	repo := &fakeScanRepo{
		subs:        []models.GitHubRelease{{Repo: "owner/repo", LastSeenTag: "v1.0.0"}},
		subscribers: []string{"user@example.com"},
		updateErr:   errors.New("db unavailable"),
	}
	scanner, jobs := newTestScanner(repo, fakeTagFetcher{tag: "v2.0.0"})

	if err := scanner.scan(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(jobs) != 0 {
		t.Errorf("jobs = %d, want 0 (notification must not fire if DB update failed)", len(jobs))
	}
}
