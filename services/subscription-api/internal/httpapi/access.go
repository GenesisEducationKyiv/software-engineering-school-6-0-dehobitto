package httpapi

import (
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"

	"subber/pkg/logger"
	"subber/pkg/requestid"
)

type AccessMetrics struct {
	Requests *prometheus.CounterVec
	Duration *prometheus.HistogramVec
}

func NewAccessMetrics() *AccessMetrics {
	return &AccessMetrics{
		Requests: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "subber_http_requests_total",
				Help: "Total HTTP requests handled by subscription-api.",
			},
			[]string{"method", "route", "status"},
		),
		Duration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "subber_http_request_duration_seconds",
				Help:    "HTTP request duration in seconds for subscription-api.",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"method", "route"},
		),
	}
}

func accessMiddleware(log logger.Logger, metrics *AccessMetrics) gin.HandlerFunc {
	if log == nil {
		log = logger.NewNoop()
	}

	return func(c *gin.Context) {
		path := c.Request.URL.Path
		if path != "/api" && !strings.HasPrefix(path, "/api/") {
			c.Next()
			return
		}

		started := time.Now()
		c.Next()

		route := c.FullPath()
		if route == "" {
			route = "unmatched"
		}
		method := c.Request.Method
		status := c.Writer.Status()
		elapsed := time.Since(started)

		if metrics != nil {
			if metrics.Requests != nil {
				metrics.Requests.WithLabelValues(method, route, strconv.Itoa(status)).Inc()
			}
			if metrics.Duration != nil {
				metrics.Duration.WithLabelValues(method, route).Observe(elapsed.Seconds())
			}
		}

		entry := log.
			WithField("method", method).
			WithField("route", route).
			WithField("status", status).
			WithField("duration_ms", elapsed.Milliseconds())

		if requestID, ok := requestid.FromContext(c.Request.Context()); ok {
			entry = entry.WithField("request_id", requestID)
		}

		entry.Info("request completed")
	}
}
