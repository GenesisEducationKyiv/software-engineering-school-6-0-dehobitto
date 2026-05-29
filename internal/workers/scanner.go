package workers

import (
	"context"
	"fmt"
	"log"
	"time"

	"subber/internal/metrics"
	"subber/internal/models"
)

type ScanRepository interface {
	GetUniqueSubscriptions(ctx context.Context) ([]models.GitHubRelease, error)
	GetSubscribers(ctx context.Context, repo string) ([]string, error)
	UpdateTags(ctx context.Context, repo models.GitHubRelease) error
}

type ReleaseChecker interface {
	GetLatestTag(ctx context.Context, repo string) (string, error)
}

type ScannerWorker struct {
	repo   ScanRepository
	jobs   chan<- NotificationJob
	github ReleaseChecker
}

func NewScannerWorker(repo ScanRepository, jobs chan<- NotificationJob, gh ReleaseChecker) *ScannerWorker {
	return &ScannerWorker{
		repo:   repo,
		jobs:   jobs,
		github: gh,
	}
}

func (w *ScannerWorker) StartScanner(ctx context.Context) error {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			scanCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			err := w.scan(scanCtx)
			cancel()
			if err != nil {
				log.Printf("Scan failed: %v", err)
			}
			metrics.ScanCyclesTotal.Inc()
		}
	}
}

func (w *ScannerWorker) scan(ctx context.Context) error {
	repos, err := w.repo.GetUniqueSubscriptions(ctx)
	if err != nil {
		return fmt.Errorf("query unique repos failed: %w", err)
	}

	updated := w.checkForNewReleases(ctx, repos)
	w.persistAndNotify(ctx, updated)
	return nil
}

// checkForNewReleases polls GitHub for each repo and returns only those with a new tag.
func (w *ScannerWorker) checkForNewReleases(ctx context.Context, repos []models.GitHubRelease) []models.GitHubRelease {
	var updated []models.GitHubRelease

	for _, repo := range repos {
		newTag, err := w.github.GetLatestTag(ctx, repo.Repo)
		if err != nil {
			log.Printf("failed to get tag for %s: %v", repo.Repo, err)
			continue
		}

		if newTag != "" && newTag != repo.LastSeenTag {
			repo.LastSeenTag = newTag
			updated = append(updated, repo)
		}
	}

	return updated
}

// persistAndNotify saves new tags to the database and enqueues notification jobs.
func (w *ScannerWorker) persistAndNotify(ctx context.Context, repos []models.GitHubRelease) {
	for _, repo := range repos {
		if err := w.repo.UpdateTags(ctx, repo); err != nil {
			log.Printf("failed to update tag in db for %s: %v", repo.Repo, err)
			continue
		}

		w.enqueueNotifications(repo)
	}
}

// enqueueNotifications fetches subscribers for a repo and pushes a job per subscriber.
func (w *ScannerWorker) enqueueNotifications(repo models.GitHubRelease) {
	// Use a background context so a cancelled scan context doesn't drop notifications.
	emails, err := w.repo.GetSubscribers(context.Background(), repo.Repo)
	if err != nil {
		log.Printf("failed to get subscribers for %s: %v", repo.Repo, err)
		return
	}

	for _, email := range emails {
		w.jobs <- NotificationJob{
			Email:   email,
			Message: fmt.Sprintf("New release %s for %s!\n", repo.LastSeenTag, repo.Repo),
		}
	}
}
