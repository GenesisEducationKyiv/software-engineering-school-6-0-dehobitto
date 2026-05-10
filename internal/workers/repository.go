package workers

import (
	"context"

	"subber/internal/models"
)

// ScanRepository is the persistence contract required by ScannerWorker.
type ScanRepository interface {
	GetUniqueSubscriptions(ctx context.Context) ([]models.GitHubRelease, error)
	GetSubscribers(ctx context.Context, repo string) ([]string, error)
	UpdateTags(ctx context.Context, repo models.GitHubRelease) error
}
