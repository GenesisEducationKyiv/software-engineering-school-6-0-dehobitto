// Package service contains business logic for subscription management.
package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"

	"subber/internal/config"
	"subber/internal/infra/cache"
	"subber/internal/models"
	"subber/internal/workers"
)

var (
	ErrAlreadySubscribed = errors.New("already subscribed")
	ErrRepoNotFound      = errors.New("repository not found")
	ErrGitHubRateLimit   = errors.New("github rate limit exceeded")
	ErrGitHubUnavailable = errors.New("github unavailable")
)

type gitHubClient interface {
	CheckIfRepoExists(ctx context.Context, repo, token string) (*http.Response, error)
	GetLatestTag(ctx context.Context, repo, token string, rc cache.Cache) (string, error)
}

type SubscriptionService struct {
	repo    SubscriptionRepository
	cfg     *config.Config
	jobs    chan<- workers.NotificationJob
	cache   cache.Cache
	github  gitHubClient
	uuidGen UUIDGenerator
}

func NewSubscriptionService(repo SubscriptionRepository, cfg *config.Config, jobs chan<- workers.NotificationJob, cache cache.Cache, gh gitHubClient, uuidGen UUIDGenerator) *SubscriptionService {
	return &SubscriptionService{
		repo:    repo,
		cfg:     cfg,
		jobs:    jobs,
		cache:   cache,
		github:  gh,
		uuidGen: uuidGen,
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

	tag, err := s.github.GetLatestTag(ctx, repo, s.cfg.GitHubToken, s.cache)
	if err != nil {
		log.Printf("Warning: could not fetch initial tag for %s: %v", repo, err)
	}

	sub := models.Subscription{
		Email:       email,
		Repo:        repo,
		LastSeenTag: tag,
		Token:       s.uuidGen.New(),
		Confirmed:   false,
	}

	if err := s.repo.SaveSubscription(ctx, sub); err != nil {
		return fmt.Errorf("save subscription: %w", err)
	}

	s.enqueueConfirmation(sub.Email, sub.Token)
	return nil
}

func (s *SubscriptionService) validateRepoOnGitHub(ctx context.Context, repo string) error {
	resp, err := s.github.CheckIfRepoExists(ctx, repo, s.cfg.GitHubToken)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrGitHubUnavailable, err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Printf("failed to close github response body: %v", err)
		}
	}()

	switch resp.StatusCode {
	case http.StatusOK:
		return nil
	case http.StatusNotFound:
		return ErrRepoNotFound
	case http.StatusTooManyRequests:
		return ErrGitHubRateLimit
	default:
		return ErrGitHubUnavailable
	}
}

func (s *SubscriptionService) enqueueConfirmation(email, token string) {
	confirmURL := fmt.Sprintf("%s/api/confirm/%s", s.cfg.BaseURL, token)
	message := fmt.Sprintf(
		"Welcome! Please confirm your subscription to GitHub repository updates by clicking here: %s",
		confirmURL,
	)

	select {
	case s.jobs <- workers.NotificationJob{Email: email, Message: message}:
		log.Printf("Confirmation job queued for: %s", email)
	default:
		log.Printf("Critical: notification channel full, dropping confirmation for: %s", email)
	}
}
