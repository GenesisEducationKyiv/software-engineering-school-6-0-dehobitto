// Package handlers provides functions for all API endpoints
package handlers

import (
	"subber/internal/service"
)

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
