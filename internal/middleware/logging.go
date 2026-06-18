package middleware

import (
	"time"

	"subber/internal/logger"

	"github.com/gin-gonic/gin"
)

func LoggingMiddleware(log logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		c.Next()

		duration := time.Since(start).Milliseconds()
		statusCode := c.Writer.Status()
		route := c.FullPath()
		if route == "" {
			route = "unmatched"
		}

		entry := logger.WithRequestID(log, c.Request.Context()).
			WithField("method", c.Request.Method).
			WithField("route", route).
			WithField("status_code", statusCode).
			WithField("duration_ms", duration).
			WithField("has_query", c.Request.URL.RawQuery != "").
			WithField("user_agent", c.Request.UserAgent())

		if ipHash := logger.IPHash(c.ClientIP()); ipHash != "" {
			entry = entry.WithField("client_ip_hash", ipHash)
		}

		switch {
		case statusCode >= 500:
			entry.Error("request")
		case statusCode >= 400:
			entry.Warn("request")
		default:
			entry.Info("request")
		}
	}
}
