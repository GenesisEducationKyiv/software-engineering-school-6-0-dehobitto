// Package service contains business logic for subscription management.
package service

import (
	"context"
	"errors"
	"fmt"

	"subber/internal/github"
	"subber/internal/logger"
	"subber/internal/models"
)

var svcLog = logger.New().WithField("component", "service")

type SubscriptionRepository interface {
	SubscriptionExists(ctx context.Context, email, repo string) (bool, error)
	SaveSubscription(ctx context.Context, sub models.Subscription) error
}

type GitHubClient interface {
	GetLatestTag(ctx context.Context, repo string) (string, error)
	CheckIfRepoExists(ctx context.Context, repo string) error
}

var (
	ErrAlreadySubscribed = errors.New("already subscribed")
	ErrRepoNotFound      = errors.New("repository not found")
	ErrGitHubRateLimit   = errors.New("github rate limit exceeded")
	ErrGitHubUnavailable = errors.New("github unavailable")
)

type SubscriptionService struct {
	repo    SubscriptionRepository
	baseURL string
	jobs    chan<- models.NotificationJob
	github  GitHubClient
	gen     UUIDGenerator
}

func NewSubscriptionService(repo SubscriptionRepository, baseURL string, jobs chan<- models.NotificationJob, gh GitHubClient, gen UUIDGenerator) *SubscriptionService {
	return &SubscriptionService{
		repo:    repo,
		baseURL: baseURL,
		jobs:    jobs,
		github:  gh,
		gen:     gen,
	}
}

// Subscribe runs the full subscription flow: dedup check, GitHub validation, persist, confirmation email.
func (s *SubscriptionService) Subscribe(ctx context.Context, email, repo string) error {
	exists, err := s.repo.SubscriptionExists(ctx, email, repo)
	if err != nil {
		return fmt.Errorf("check subscription: %w", err)
	}
	if exists {
		return ErrAlreadySubscribed
	}

	if err := s.validateRepoOnGitHub(ctx, repo); err != nil {
		return err
	}

	tag, err := s.github.GetLatestTag(ctx, repo)
	if err != nil {
		svcLog.WithField("repo", repo).WithError(err).Warn("could not fetch initial tag")
	}

	sub := models.Subscription{
		Email:       email,
		Repo:        repo,
		LastSeenTag: tag,
		Token:       s.gen.New(),
		Confirmed:   false,
	}

	if err := s.repo.SaveSubscription(ctx, sub); err != nil {
		return fmt.Errorf("save subscription: %w", err)
	}

	s.enqueueConfirmation(sub.Email, sub.Repo, sub.Token)
	return nil
}

func (s *SubscriptionService) validateRepoOnGitHub(ctx context.Context, repo string) error {
	err := s.github.CheckIfRepoExists(ctx, repo)
	switch {
	case err == nil:
		return nil
	case errors.Is(err, github.ErrNotFound):
		return ErrRepoNotFound
	case errors.Is(err, github.ErrRateLimit):
		return ErrGitHubRateLimit
	default:
		return fmt.Errorf("%w: %w", ErrGitHubUnavailable, err)
	}
}

func (s *SubscriptionService) enqueueConfirmation(email, repo, token string) {
	confirmURL := fmt.Sprintf("%s/api/confirm/%s", s.baseURL, token)
	message := fmt.Sprintf(
		"Welcome! Please confirm your subscription to GitHub repository updates by clicking here: %s",
		confirmURL,
	)

	select {
	case s.jobs <- models.NotificationJob{Email: email, Repo: repo, Message: message}:
		svcLog.WithField("email", email).WithField("repo", repo).Info("confirmation job queued")
	default:
		svcLog.WithField("email", email).Error("notification channel full, dropping confirmation")
	}
}
