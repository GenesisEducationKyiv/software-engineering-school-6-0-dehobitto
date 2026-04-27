package workers

import (
	"context"
	"fmt"
	"log"
	"time"

	"subber/config"
	"subber/infra/cache"
	"subber/infra/database"
	"subber/models"
	"subber/utils/github"
)

type ScannerWorker struct {
	repo  *database.Repository
	cfg   *config.Config
	jobs  chan<- NotificationJob
	cache *cache.RedisCache
}

func NewScannerWorker(repo *database.Repository, cfg *config.Config, jobs chan<- NotificationJob, rc *cache.RedisCache) *ScannerWorker {
	return &ScannerWorker{
		repo:  repo,
		cfg:   cfg,
		jobs:  jobs,
		cache: rc,
	}
}

func (w *ScannerWorker) StartScanner() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		func() {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			err := w.scan(ctx)
			if err != nil {
				log.Printf("Scan failed: %v", err)
			}
		}()
	}
}

func (w *ScannerWorker) scan(ctx context.Context) error {
	uniqueRepos, err := w.repo.GetUniqueSubscriptions(ctx)
	if err != nil {
		return fmt.Errorf("query unique repos failed: %w", err)
	}

	var updatedRepos []models.GitHubRelease

	for _, repo := range uniqueRepos {
		newTag, err := github.GetLatestTag(ctx, repo.Repo, w.cfg.GitHubToken, w.cache)
		if err != nil {
			log.Printf("failed to get tag for %s: %v", repo.Repo, err)
			continue
		}

		if newTag != "" && newTag != repo.LastSeenTag {
			repo.LastSeenTag = newTag
			updatedRepos = append(updatedRepos, repo)
		}
	}

	for _, repo := range updatedRepos {
		err := w.repo.UpdateTags(ctx, repo)
		if err != nil {
			log.Printf("failed to update tag in db for %s: %v", repo.Repo, err)
			continue
		}

		emails, err := w.repo.GetSubscribers(ctx, repo.Repo)
		if err != nil {
			log.Printf("failed to get subscribers for %s: %v", repo.Repo, err)
			continue
		}

		for _, email := range emails {
			msg := fmt.Sprintf("New release %s for %s!\n", repo.LastSeenTag, repo.Repo)

			w.jobs <- NotificationJob{
				Email:   email,
				Message: msg,
			}
		}
	}

	return nil
}
