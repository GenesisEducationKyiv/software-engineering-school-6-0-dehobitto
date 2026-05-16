package handlers

import (
	"context"

	"subber/internal/service"
)

type subscriptionService interface {
	Subscribe(ctx context.Context, email, repo string) error
}

type Handler struct {
	repo SubscriptionRepository
	svc  subscriptionService
}

func NewHandler(repo SubscriptionRepository, svc *service.SubscriptionService) *Handler {
	return &Handler{
		repo: repo,
		svc:  svc,
	}
}
