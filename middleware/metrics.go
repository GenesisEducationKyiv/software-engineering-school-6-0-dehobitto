package middleware

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	httpRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "path", "status"},
	)

	httpRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "Duration of HTTP requests in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)

	EmailsSentTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "emails_sent_total",
			Help: "Total number of emails sent",
		},
	)

	EmailsFailedTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "emails_failed_total",
			Help: "Total number of failed email sends",
		},
	)

	ScanCyclesTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "scan_cycles_total",
			Help: "Total number of scan cycles completed",
		},
	)
)

func PrometheusMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		c.Next()

		duration := time.Since(start).Seconds()
		status := strconv.Itoa(c.Writer.Status())

		httpRequestsTotal.WithLabelValues(c.Request.Method, c.FullPath(), status).Inc()
		httpRequestDuration.WithLabelValues(c.Request.Method, c.FullPath()).Observe(duration)
	}
}
