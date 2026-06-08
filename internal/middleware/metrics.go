package middleware

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"subber/internal/metrics"
)

func PrometheusMiddleware(appMetrics *metrics.Metrics) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		c.Next()

		duration := time.Since(start).Seconds()
		status := strconv.Itoa(c.Writer.Status())
		route := c.FullPath()
		if route == "" {
			route = "unmatched"
		}

		appMetrics.HTTPRequestsTotal.WithLabelValues(c.Request.Method, route, status).Inc()
		appMetrics.HTTPRequestDuration.WithLabelValues(c.Request.Method, route).Observe(duration)
	}
}
