package workers

import (
	"context"
	"errors"
	"testing"

	"subber/internal/config"
	"subber/internal/infra/cache"
	"subber/internal/models"
)

type fakeTagFetcher struct {
	tag string
	err error
}

func (f fakeTagFetcher) GetLatestTag(_ context.Context, _, _ string, _ cache.Cache) (string, error) {
	return f.tag, f.err
}

type fakeScanRepo struct {
	subs        []models.GitHubRelease
	subscribers []string
	updatedRepo string
}

func (r *fakeScanRepo) GetUniqueSubscriptions(_ context.Context) ([]models.GitHubRelease, error) {
	return r.subs, nil
}

func (r *fakeScanRepo) GetSubscribers(_ context.Context, _ string) ([]string, error) {
	return r.subscribers, nil
}

func (r *fakeScanRepo) UpdateTags(_ context.Context, repo models.GitHubRelease) error {
	r.updatedRepo = repo.Repo
	return nil
}

func newTestScanner(repo *fakeScanRepo, gh githubTagFetcher) (*ScannerWorker, chan NotificationJob) {
	jobs := make(chan NotificationJob, 10)
	cfg := &config.Config{}
	return &ScannerWorker{repo: repo, cfg: cfg, jobs: jobs, github: gh}, jobs
}

func TestScanner_SkipsRepo_WhenGitHubFails(t *testing.T) {
	repo := &fakeScanRepo{
		subs: []models.GitHubRelease{{Repo: "owner/repo", LastSeenTag: "v1.0.0"}},
	}
	scanner, jobs := newTestScanner(repo, fakeTagFetcher{err: errors.New("timeout")})

	if err := scanner.scan(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(jobs) != 0 {
		t.Errorf("jobs = %d, want 0 (github error must not trigger notification)", len(jobs))
	}
}

func TestScanner_SkipsJob_WhenTagEmpty(t *testing.T) {
	repo := &fakeScanRepo{
		subs: []models.GitHubRelease{{Repo: "owner/repo", LastSeenTag: "v1.0.0"}},
	}
	scanner, jobs := newTestScanner(repo, fakeTagFetcher{tag: ""})

	if err := scanner.scan(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(jobs) != 0 {
		t.Errorf("jobs = %d, want 0 (empty tag must not trigger notification)", len(jobs))
	}
}

func TestScanner_SkipsJob_WhenTagUnchanged(t *testing.T) {
	repo := &fakeScanRepo{
		subs: []models.GitHubRelease{{Repo: "owner/repo", LastSeenTag: "v1.0.0"}},
	}
	scanner, jobs := newTestScanner(repo, fakeTagFetcher{tag: "v1.0.0"})

	if err := scanner.scan(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(jobs) != 0 {
		t.Errorf("jobs = %d, want 0 (same tag must not trigger notification)", len(jobs))
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
}
