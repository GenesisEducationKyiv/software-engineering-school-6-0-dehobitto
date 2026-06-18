// Package routes defines HTTP routing and endpoint registration.
package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"subber/internal/config"
	"subber/internal/handlers"
	"subber/internal/logger"
	"subber/internal/metrics"
	"subber/internal/middleware"
)

func SetupRouter(repo handlers.SubscriptionRepository, svc handlers.SubscriptionService, cfg *config.Config, log logger.Logger, appMetrics *metrics.Metrics, gatherer prometheus.Gatherer) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())
	r.SetTrustedProxies(nil) //nolint:gosec // nil input cannot produce a parse error

	r.Use(middleware.RequestIDMiddleware())
	r.Use(middleware.PrometheusMiddleware(appMetrics))
	r.Use(middleware.LoggingMiddleware(log.WithField("component", "http")))

	r.GET("/metrics", gin.WrapH(promhttp.HandlerFor(gatherer, promhttp.HandlerOpts{})))

	r.Static("/static", "./static")
	r.GET("/", func(c *gin.Context) {
		c.File("./static/index.html")
	})

	h := handlers.NewHandler(repo, svc, log.WithField("component", "handler"))

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
