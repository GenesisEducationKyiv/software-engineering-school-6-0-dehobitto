package scanner

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"subber/pkg/contracts"
	"subber/pkg/logger"
)

type ReleaseProvider interface {
	GetLatestTag(ctx context.Context, repo string) (string, error)
}

type Store interface {
	ClaimDue(ctx context.Context, limit int, nextScanIn time.Duration) ([]WatchedRepo, error)
	StartWatching(ctx context.Context, repo string) error
	StopWatching(ctx context.Context, repo string) error
	MarkReleaseDetected(ctx context.Context, repo, tag, correlationID string) (bool, error)
}

type Service struct {
	repo        Store
	releases    ReleaseProvider
	log         logger.Logger
	batchSize   int
	scanBackoff time.Duration
}

func NewService(repo Store, releases ReleaseProvider, log logger.Logger, batchSize int, scanBackoff time.Duration) *Service {
	if log == nil {
		log = logger.NewNoop()
	}
	if batchSize <= 0 {
		batchSize = 100
	}
	if scanBackoff <= 0 {
		scanBackoff = 30 * time.Second
	}
	return &Service{
		repo:        repo,
		releases:    releases,
		log:         log,
		batchSize:   batchSize,
		scanBackoff: scanBackoff,
	}
}

func (s *Service) Start(ctx context.Context) error {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := s.ScanOnce(ctx); err != nil {
				s.log.WithError(err).Error("scanner batch failed")
			}
		}
	}
}

func (s *Service) ScanOnce(ctx context.Context) error {
	repos, err := s.repo.ClaimDue(ctx, s.batchSize, s.scanBackoff)
	if err != nil {
		return err
	}
	if len(repos) == 0 {
		return nil
	}

	correlationID := uuid.NewString()
	for _, watched := range repos {
		if err := s.scanRepo(ctx, watched, correlationID); err != nil {
			s.log.WithField("repo", watched.Repo).WithError(err).Error("repo scan failed")
		}
	}
	return nil
}

func (s *Service) HandleWatchlistEvent(ctx context.Context, value []byte) error {
	var event contracts.Envelope[contracts.RepoWatchPayload]
	if err := json.Unmarshal(value, &event); err != nil {
		return fmt.Errorf("decode watchlist event: %w", err)
	}

	switch event.EventType {
	case contracts.EventRepoWatchStart:
		return s.repo.StartWatching(ctx, event.Payload.Repo)
	case contracts.EventRepoWatchStop:
		return s.repo.StopWatching(ctx, event.Payload.Repo)
	default:
		return fmt.Errorf("unsupported watchlist event type %q", event.EventType)
	}
}

func (s *Service) scanRepo(ctx context.Context, watched WatchedRepo, correlationID string) error {
	tag, err := s.releases.GetLatestTag(ctx, watched.Repo)
	if err != nil {
		return fmt.Errorf("get latest tag: %w", err)
	}
	if tag == "" || tag == watched.LastSeenTag {
		return nil
	}

	published, err := s.repo.MarkReleaseDetected(ctx, watched.Repo, tag, correlationID)
	if err != nil {
		return err
	}
	if published {
		s.log.WithField("repo", watched.Repo).WithField("tag", tag).Info("release detected")
	}
	return nil
}
