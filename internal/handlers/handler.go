package handlers

import (
	"subber/internal/config"
	"subber/internal/infra/cache"
	"subber/internal/infra/database"
	"subber/internal/workers"
)

type Handler struct {
	repo  *database.Repository
	cfg   *config.Config
	jobs  chan<- workers.NotificationJob
	cache *cache.RedisCache
}

func NewHandler(repo *database.Repository, cfg *config.Config, jobs chan<- workers.NotificationJob, rc *cache.RedisCache) *Handler {
	return &Handler{
		repo:  repo,
		cfg:   cfg,
		jobs:  jobs,
		cache: rc,
	}
}
