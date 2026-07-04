package subscription

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"subber/pkg/logger"
)

var (
	ErrAlreadySubscribed = errors.New("already subscribed")
	ErrRepoNotFound      = errors.New("repository not found")
	ErrGitHubRateLimit   = errors.New("github rate limit exceeded")
	ErrGitHubUnavailable = errors.New("github unavailable")
)

type NotificationPublisher interface {
	PublishConfirmationTx(ctx context.Context, tx pgx.Tx, email, repo, token string) error
}

type Store interface {
	SubscriptionExists(ctx context.Context, email, repo string) (bool, error)
	SaveSubscriptionWithConfirmation(ctx context.Context, sub Subscription, publisher NotificationPublisher) error
}

type Service struct {
	repo          Store
	notifications NotificationPublisher
	github        GitHubClient
	log           logger.Logger
}

func NewService(repo Store, notifications NotificationPublisher, github GitHubClient, log logger.Logger) *Service {
	if log == nil {
		log = logger.NewNoop()
	}
	return &Service{repo: repo, notifications: notifications, github: github, log: log}
}

func (s *Service) Subscribe(ctx context.Context, email, repo string) error {
	exists, err := s.repo.SubscriptionExists(ctx, email, repo)
	if err != nil {
		return fmt.Errorf("check subscription: %w", err)
	}
	if exists {
		return ErrAlreadySubscribed
	}

	if err := s.validateRepo(ctx, repo); err != nil {
		return err
	}

	tag, err := s.github.GetLatestTag(ctx, repo)
	if err != nil {
		s.log.WithField("repo", repo).WithError(err).Warn("could not fetch initial tag")
	}

	sub := Subscription{
		Email:       email,
		Repo:        repo,
		LastSeenTag: tag,
		Token:       uuid.NewString(),
		Confirmed:   false,
	}
	return s.repo.SaveSubscriptionWithConfirmation(ctx, sub, s.notifications)
}

func (s *Service) validateRepo(ctx context.Context, repo string) error {
	err := s.github.CheckIfRepoExists(ctx, repo)
	switch {
	case err == nil:
		return nil
	case errors.Is(err, ErrGitHubNotFound):
		return ErrRepoNotFound
	case errors.Is(err, ErrGitHubAPILimit):
		return ErrGitHubRateLimit
	default:
		return fmt.Errorf("%w: %w", ErrGitHubUnavailable, err)
	}
}
