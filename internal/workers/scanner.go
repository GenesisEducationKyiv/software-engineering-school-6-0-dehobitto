package workers

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"subber/internal/logger"
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
	jobs   chan<- models.NotificationJob
	github ReleaseChecker
	log    logger.Logger
	met    *metrics.Metrics
}

func NewScannerWorker(repo ScanRepository, jobs chan<- models.NotificationJob, gh ReleaseChecker, log logger.Logger, appMetrics *metrics.Metrics) *ScannerWorker {
	if log == nil {
		log = logger.NewNoop()
	}
	if appMetrics == nil {
		appMetrics = metrics.NewNoop()
	}
	return &ScannerWorker{
		repo:   repo,
		jobs:   jobs,
		github: gh,
		log:    log,
		met:    appMetrics,
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
			scanCycleID := uuid.NewString()
			err := w.scan(scanCtx, scanCycleID)
			cancel()
			if err != nil {
				w.log.WithField("scan_cycle_id", scanCycleID).WithError(err).Error("scan cycle failed")
			}
			w.met.ScanCyclesTotal.Inc()
		}
	}
}

func (w *ScannerWorker) scan(ctx context.Context, scanCycleID string) error {
	log := w.log.WithField("scan_cycle_id", scanCycleID)
	repos, err := w.repo.GetUniqueSubscriptions(ctx)
	if err != nil {
		return fmt.Errorf("query unique repos failed: %w", err)
	}

	updated := w.checkForNewReleases(ctx, repos, log)
	w.persistAndNotify(ctx, updated, scanCycleID, log)
	return nil
}

// checkForNewReleases polls GitHub for each repo and returns only those with a new tag.
func (w *ScannerWorker) checkForNewReleases(ctx context.Context, repos []models.GitHubRelease, log logger.Logger) []models.GitHubRelease {
	var updated []models.GitHubRelease

	for _, repo := range repos {
		newTag, err := w.github.GetLatestTag(ctx, repo.Repo)
		if err != nil {
			log.WithField("repo", repo.Repo).WithError(err).Error("failed to get tag")
			continue
		}

		if newTag != "" && newTag != repo.LastSeenTag {
			log.WithField("repo", repo.Repo).WithField("tag", newTag).Info("new release detected")
			repo.LastSeenTag = newTag
			updated = append(updated, repo)
		}
	}

	return updated
}

// persistAndNotify saves new tags to the database and enqueues notification jobs.
func (w *ScannerWorker) persistAndNotify(ctx context.Context, repos []models.GitHubRelease, scanCycleID string, log logger.Logger) {
	for _, repo := range repos {
		if err := w.repo.UpdateTags(ctx, repo); err != nil {
			log.WithField("repo", repo.Repo).WithError(err).Error("failed to update tag in db")
			continue
		}

		w.enqueueNotifications(repo, scanCycleID, log)
	}
}

// enqueueNotifications fetches subscribers for a repo and pushes a job per subscriber.
func (w *ScannerWorker) enqueueNotifications(repo models.GitHubRelease, scanCycleID string, log logger.Logger) {
	// Use a background context so a cancelled scan context doesn't drop notifications.
	emails, err := w.repo.GetSubscribers(context.Background(), repo.Repo)
	if err != nil {
		log.WithField("repo", repo.Repo).WithError(err).Error("failed to get subscribers")
		return
	}

	for _, email := range emails {
		w.jobs <- models.NotificationJob{
			Email:       email,
			Repo:        repo.Repo,
			Message:     fmt.Sprintf("New release %s for %s!\n", repo.LastSeenTag, repo.Repo),
			ScanCycleID: scanCycleID,
		}
	}
}
