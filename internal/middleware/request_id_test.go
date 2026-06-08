package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"subber/internal/requestid"
)

func TestRequestIDMiddlewareGeneratesID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RequestIDMiddleware())
	r.GET("/ping", func(c *gin.Context) {
		if _, ok := requestid.FromContext(c.Request.Context()); !ok {
			t.Fatal("request id missing from context")
		}
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Header().Get(requestid.Header) == "" {
		t.Fatal("request id missing from response header")
	}
}

func TestRequestIDMiddlewarePropagatesValidHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RequestIDMiddleware())
	r.GET("/ping", func(c *gin.Context) {
		got, _ := requestid.FromContext(c.Request.Context())
		if got != "client-request-1" {
			t.Fatalf("request id = %q, want client-request-1", got)
		}
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	req.Header.Set(requestid.Header, "client-request-1")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if got := w.Header().Get(requestid.Header); got != "client-request-1" {
		t.Fatalf("response request id = %q, want client-request-1", got)
	}
}
