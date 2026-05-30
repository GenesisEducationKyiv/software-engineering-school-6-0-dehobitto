package middleware

import (
	"time"

	"subber/internal/logger"

	"github.com/gin-gonic/gin"
)

var httpLog = logger.New().WithField("component", "http")

func LoggingMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		c.Next()

		duration := time.Since(start).Milliseconds()
		statusCode := c.Writer.Status()

		entry := httpLog.
			WithField("method", c.Request.Method).
			WithField("path", c.FullPath()).
			WithField("status_code", statusCode).
			WithField("duration_ms", duration).
			WithField("ip", c.ClientIP())

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
