package routes

import (
	"subber/config"
	"subber/handlers"
	"subber/infra/cache"
	"subber/infra/database"
	"subber/middleware"
	"subber/workers"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func SetupRouter(repo *database.Repository, cfg *config.Config, jobs chan<- workers.NotificationJob, rc *cache.RedisCache) *gin.Engine {
	r := gin.Default()
	_ = r.SetTrustedProxies(nil)

	r.Use(middleware.PrometheusMiddleware())

	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	r.Static("/static", "./static")
	r.GET("/", func(c *gin.Context) {
		c.File("./static/index.html")
	})

	h := handlers.NewHandler(repo, cfg, jobs, rc)

	api := r.Group("/api")
	{
		api.GET("/confirm/:token", h.ConfirmByToken)
		api.GET("/unsubscribe/:token", h.UnsubscribeByToken)
	}

	protected := api.Group("/")
	protected.Use(middleware.APIKeyAuth(cfg.APIKey))
	{
		protected.POST("/subscribe", h.Subscribe)
		protected.GET("/subscriptions/", h.GetSubscriptions)
	}

	return r
}
