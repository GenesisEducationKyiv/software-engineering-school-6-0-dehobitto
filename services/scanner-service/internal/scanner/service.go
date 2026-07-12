package scanner

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"subber/pkg/contracts"
	"subber/pkg/kafka"
	"subber/pkg/logger"
)

type ReleaseProvider interface {
	GetLatestTag(ctx context.Context, repo string) (string, error)
}

type Store interface {
	ClaimDue(ctx context.Context, limit int, nextScanIn time.Duration) ([]WatchedRepo, error)
	ApplyWatchCommand(ctx context.Context, payload contracts.RepoWatchCommandPayload, correlationID string) error
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

func (s *Service) HandleWatchlistCommand(ctx context.Context, value []byte) error {
	var event contracts.Envelope[contracts.RepoWatchCommandPayload]
	if err := json.Unmarshal(value, &event); err != nil {
		return kafka.NonRetryable(fmt.Errorf("decode watchlist command: %w", err))
	}

	switch event.EventType {
	case contracts.EventStartWatchingRepo:
		if event.Payload.Action != contracts.RepoWatchActionStart {
			return kafka.NonRetryable(fmt.Errorf("start command action = %q", event.Payload.Action))
		}
	case contracts.EventStopWatchingRepo:
		if event.Payload.Action != contracts.RepoWatchActionStop {
			return kafka.NonRetryable(fmt.Errorf("stop command action = %q", event.Payload.Action))
		}
	default:
		return kafka.NonRetryable(fmt.Errorf("unsupported watchlist command type %q", event.EventType))
	}
	if event.Payload.SagaID == "" || event.Payload.Repo == "" {
		return kafka.NonRetryable(fmt.Errorf("watchlist command requires saga_id and repo"))
	}
	return s.repo.ApplyWatchCommand(ctx, event.Payload, event.CorrelationID)
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
