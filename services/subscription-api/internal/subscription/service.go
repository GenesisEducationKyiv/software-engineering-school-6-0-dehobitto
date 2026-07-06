package subscription

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"subber/pkg/contracts"
	"subber/pkg/logger"
)

var (
	ErrAlreadySubscribed       = errors.New("already subscribed")
	ErrRepoNotFound            = errors.New("repository not found")
	ErrGitHubRateLimit         = errors.New("github rate limit exceeded")
	ErrGitHubUnavailable       = errors.New("github unavailable")
	ErrTokenNotFound           = errors.New("token not found")
	ErrConfirmationUnavailable = errors.New("confirmation notification unavailable")
)

type NotificationPublisher interface {
	SendConfirmation(ctx context.Context, email, repo, token string) error
}

type Store interface {
	SubscriptionExists(ctx context.Context, email, repo string) (bool, error)
	SaveSubscription(ctx context.Context, sub Subscription) error
	DeleteUnconfirmedSubscription(ctx context.Context, email, repo, token string) error
	ConfirmSubscriptionByToken(ctx context.Context, token string) (ConfirmSubscriptionResult, error)
	RequestRepoWatchSaga(ctx context.Context, action, repo, email string) error
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
		return err
	}
	s.log.WithField("repo", repo).WithField("email", email).Info("subscription saga step completed: unconfirmed subscription created")

	if err := s.notifications.SendConfirmation(ctx, sub.Email, sub.Repo, sub.Token); err != nil {
		s.log.WithField("repo", repo).WithField("email", email).WithError(err).Warn("subscription saga step failed: confirmation notification")
		if compensationErr := s.repo.DeleteUnconfirmedSubscription(ctx, sub.Email, sub.Repo, sub.Token); compensationErr != nil {
			return fmt.Errorf("%w: send confirmation: %w; compensate subscription: %w", ErrConfirmationUnavailable, err, compensationErr)
		}
		s.log.WithField("repo", repo).WithField("email", email).Info("subscription saga compensation completed: unconfirmed subscription deleted")
		return fmt.Errorf("%w: %w", ErrConfirmationUnavailable, err)
	}
	s.log.WithField("repo", repo).WithField("email", email).Info("subscription saga completed: confirmation notification sent")
	return nil
}

func (s *Service) ConfirmSubscriptionByToken(ctx context.Context, token string) error {
	result, err := s.repo.ConfirmSubscriptionByToken(ctx, token)
	if err != nil {
		return err
	}
	if !result.WasFirstConfirmed {
		return nil
	}
	return s.repo.RequestRepoWatchSaga(ctx, contracts.RepoWatchActionStart, result.Repo, result.Email)
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
