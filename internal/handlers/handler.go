package handlers

import (
	"context"

	"subber/internal/models"
	"subber/internal/service"
)

type SubscriptionRepository interface {
	ConfirmSubscriptionByToken(ctx context.Context, token string) error
	Unsubscribe(ctx context.Context, token string) error
	GetSubscriptions(ctx context.Context, email string) ([]models.Subscription, error)
}

type Handler struct {
	repo SubscriptionRepository
	svc  *service.SubscriptionService
}

func NewHandler(repo SubscriptionRepository, svc *service.SubscriptionService) *Handler {
	return &Handler{
		repo: repo,
		svc:  svc,
	}
}
