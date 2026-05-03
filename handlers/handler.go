package handlers

import (
	"subber/config"
	"subber/infra/cache"
	"subber/infra/database"
	"subber/workers"
)

// Handler provides methods to process API requests and holds application dependencies.
type Handler struct {
	repo  *database.Repository
	cfg   *config.Config
	jobs  chan<- workers.NotificationJob
	cache *cache.RedisCache
}

// NewHandler returns Handler with fields filled.
func NewHandler(repo *database.Repository, cfg *config.Config, jobs chan<- workers.NotificationJob, rc *cache.RedisCache) *Handler {
	return &Handler{
		repo:  repo,
		cfg:   cfg,
		jobs:  jobs,
		cache: rc,
	}
}
