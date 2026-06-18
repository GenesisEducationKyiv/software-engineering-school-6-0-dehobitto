// Package handlers provides functions for all API endpoints
package handlers

import (
	"context"

	"subber/internal/logger"
	"subber/internal/models"
)

type SubscriptionRepository interface {
	ConfirmSubscriptionByToken(ctx context.Context, token string) error
	Unsubscribe(ctx context.Context, token string) error
	GetSubscriptions(ctx context.Context, email string) ([]models.Subscription, error)
}

type SubscriptionService interface {
	Subscribe(ctx context.Context, email, repo string) error
}

type Handler struct {
	repo SubscriptionRepository
	svc  SubscriptionService
	log  logger.Logger
}

func NewHandler(repo SubscriptionRepository, svc SubscriptionService, log logger.Logger) *Handler {
	if log == nil {
		log = logger.NewNoop()
	}
	return &Handler{
		repo: repo,
		svc:  svc,
		log:  log,
	}
}
