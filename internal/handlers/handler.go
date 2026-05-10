package handlers

import (
	"subber/internal/infra/database"
	"subber/internal/service"
)

type Handler struct {
	repo *database.Repository
	svc  *service.SubscriptionService
}

func NewHandler(repo *database.Repository, svc *service.SubscriptionService) *Handler {
	return &Handler{
		repo: repo,
		svc:  svc,
	}
}
