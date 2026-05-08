package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func TestSubscribe_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := &Handler{}
	r.POST("/api/subscribe", h.Subscribe)

	req := httptest.NewRequest("POST", "/api/subscribe", bytes.NewBufferString("not json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestSubscribe_InvalidRepoFormat(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := &Handler{}
	r.POST("/api/subscribe", h.Subscribe)

	body, _ := json.Marshal(map[string]string{
		"email": "test@example.com",
		"repo":  "invalid-repo-format",
	})

	req := httptest.NewRequest("POST", "/api/subscribe", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestConfirmByToken_InvalidToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := &Handler{}
	r.GET("/api/confirm/:token", h.ConfirmByToken)

	req := httptest.NewRequest("GET", "/api/confirm/not-a-uuid", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestUnsubscribeByToken_InvalidToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := &Handler{}
	r.GET("/api/unsubscribe/:token", h.UnsubscribeByToken)

	req := httptest.NewRequest("GET", "/api/unsubscribe/not-valid", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestGetSubscriptions_EmptyEmail(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := &Handler{}
	r.GET("/api/subscriptions/", h.GetSubscriptions)

	req := httptest.NewRequest("GET", "/api/subscriptions/", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestGetSubscriptions_InvalidEmail(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := &Handler{}
	r.GET("/api/subscriptions/", h.GetSubscriptions)

	req := httptest.NewRequest("GET", "/api/subscriptions/?email=notanemail", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestUUIDGeneration(t *testing.T) {
	token := uuid.New().String()
	if err := uuid.Validate(token); err != nil {
		t.Errorf("generated token is not valid UUID: %s", token)
	}
}
