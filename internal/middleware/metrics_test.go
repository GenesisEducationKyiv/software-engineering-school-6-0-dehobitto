package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"subber/internal/metrics"
)

func TestPrometheusMiddlewareRecordsRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	registry := prometheus.NewRegistry()
	appMetrics := metrics.New(registry)

	r := gin.New()
	r.Use(PrometheusMiddleware(appMetrics))
	r.GET("/api/items/:id", func(c *gin.Context) {
		c.Status(http.StatusCreated)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/items/42", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	got := testutil.ToFloat64(appMetrics.HTTPRequestsTotal.WithLabelValues("GET", "/api/items/:id", "201"))
	if got != 1 {
		t.Fatalf("http_requests_total = %v, want 1", got)
	}
}

func TestPrometheusMiddlewareUsesUnmatchedRouteFor404(t *testing.T) {
	gin.SetMode(gin.TestMode)
	registry := prometheus.NewRegistry()
	appMetrics := metrics.New(registry)

	r := gin.New()
	r.Use(PrometheusMiddleware(appMetrics))

	req := httptest.NewRequest(http.MethodGet, "/missing", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	got := testutil.ToFloat64(appMetrics.HTTPRequestsTotal.WithLabelValues("GET", "unmatched", "404"))
	if got != 1 {
		t.Fatalf("http_requests_total = %v, want 1", got)
	}
}
