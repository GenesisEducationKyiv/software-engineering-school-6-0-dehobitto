package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestAPIKeyAuth(t *testing.T) {
	tests := []struct {
		name       string
		configured string
		sent       string
		want       int
	}{
		// empty configured key disables auth entirely
		{"no key configured", "", "", http.StatusOK},
		// correct key → allowed
		{"valid key", "secret", "secret", http.StatusOK},
		// wrong key → rejected
		{"wrong key", "secret", "wrong", http.StatusUnauthorized},
		// key required but header absent → rejected
		{"missing key", "secret", "", http.StatusUnauthorized},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			r := gin.New()
			r.Use(APIKeyAuth(tt.configured))
			r.GET("/test", func(c *gin.Context) { c.Status(http.StatusOK) })

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			if tt.sent != "" {
				req.Header.Set("X-API-Key", tt.sent)
			}
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != tt.want {
				t.Errorf("status = %d, want %d", w.Code, tt.want)
			}
		})
	}
}
