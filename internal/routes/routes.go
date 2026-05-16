package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"subber/internal/config"
	"subber/internal/handlers"
	"subber/internal/middleware"
	"subber/internal/service"
)

func SetupRouter(repo handlers.SubscriptionRepository, svc *service.SubscriptionService, cfg *config.Config) *gin.Engine {
	r := gin.Default()
	r.SetTrustedProxies(nil) //nolint:gosec // nil input cannot produce a parse error

	r.Use(middleware.PrometheusMiddleware())

	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	r.Static("/static", "./static")
	r.GET("/", func(c *gin.Context) {
		c.File("./static/index.html")
	})

	h := handlers.NewHandler(repo, svc)

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
