package handlers

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"subber/internal/service"
)

func TestSubscribe_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/api/subscribe", (&Handler{}).Subscribe)

	req := httptest.NewRequest("POST", "/api/subscribe", bytes.NewBufferString("not json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestSubscribe_InvalidRepoFormat(t *testing.T) {
	r := newSubscribeRouter(&fakeSvc{})

	req := httptest.NewRequest("POST", "/api/subscribe", subscribeBody(t, "test@example.com", "invalid-repo-format"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestSubscribe_ReturnsConflict_WhenAlreadySubscribed(t *testing.T) {
	r := newSubscribeRouter(&fakeSvc{err: service.ErrAlreadySubscribed})

	req := httptest.NewRequest("POST", "/api/subscribe", subscribeBody(t, "user@example.com", "owner/repo"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("status = %d, want 409", w.Code)
	}
}

func TestSubscribe_ReturnsBadGateway_WhenGitHubUnavailable(t *testing.T) {
	r := newSubscribeRouter(&fakeSvc{err: service.ErrGitHubUnavailable})

	req := httptest.NewRequest("POST", "/api/subscribe", subscribeBody(t, "user@example.com", "owner/repo"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadGateway {
		t.Errorf("status = %d, want 502", w.Code)
	}
}

func TestSubscribe_ReturnsTooManyRequests_WhenRateLimited(t *testing.T) {
	r := newSubscribeRouter(&fakeSvc{err: service.ErrGitHubRateLimit})

	req := httptest.NewRequest("POST", "/api/subscribe", subscribeBody(t, "user@example.com", "owner/repo"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("status = %d, want 429", w.Code)
	}
}

func TestSubscribe_ReturnsNotFound_WhenRepoNotFound(t *testing.T) {
	r := newSubscribeRouter(&fakeSvc{err: service.ErrRepoNotFound})

	req := httptest.NewRequest("POST", "/api/subscribe", subscribeBody(t, "user@example.com", "owner/repo"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}
