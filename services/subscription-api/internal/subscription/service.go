package subscription

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"subber/pkg/logger"
)

var (
	ErrAlreadySubscribed = errors.New("already subscribed")
	ErrRepoNotFound      = errors.New("repository not found")
	ErrGitHubRateLimit   = errors.New("github rate limit exceeded")
	ErrGitHubUnavailable = errors.New("github unavailable")
)

type NotificationPublisher interface {
	PublishConfirmation(ctx context.Context, email, repo, token string) error
}

type Store interface {
	SubscriptionExists(ctx context.Context, email, repo string) (bool, error)
	SaveSubscription(ctx context.Context, sub Subscription) error
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
	if err := s.repo.SaveSubscription(ctx, sub); err != nil {
		return fmt.Errorf("save subscription: %w", err)
	}
	return s.notifications.PublishConfirmation(ctx, sub.Email, sub.Repo, sub.Token)
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
