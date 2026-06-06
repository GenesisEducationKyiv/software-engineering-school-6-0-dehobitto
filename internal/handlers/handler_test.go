package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"subber/internal/models"
)

// fakeHandlerRepo is a test double for SubscriptionRepository.
type fakeHandlerRepo struct {
	confirmErr error
	unsubErr   error
	subs       []models.Subscription
	subsErr    error
}

func (f *fakeHandlerRepo) ConfirmSubscriptionByToken(_ context.Context, _ string) error {
	return f.confirmErr
}
func (f *fakeHandlerRepo) Unsubscribe(_ context.Context, _ string) error { return f.unsubErr }
func (f *fakeHandlerRepo) GetSubscriptions(_ context.Context, _ string) ([]models.Subscription, error) {
	return f.subs, f.subsErr
}

// fakeSvc is a test double for SubscriptionService.
type fakeSvc struct {
	err      error
	calls    int
	gotEmail string
	gotRepo  string
}

func (f *fakeSvc) Subscribe(_ context.Context, email, repo string) error {
	f.calls++
	f.gotEmail = email
	f.gotRepo = repo
	return f.err
}

// newTestRouter wires all handler routes onto a test gin engine.
func newTestRouter(repo SubscriptionRepository, svc SubscriptionService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewHandler(repo, svc)
	r.POST("/subscribe", h.Subscribe)
	r.GET("/confirm/:token", h.ConfirmByToken)
	r.GET("/unsubscribe/:token", h.UnsubscribeByToken)
	r.GET("/subscriptions/", h.GetSubscriptions)
	return r
}

// do executes a request against r and returns the response recorder.
func do(r *gin.Engine, method, path string, body []byte) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, bytes.NewBuffer(body))
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func jsonBody(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("encode json: %v", err)
	}
	return b
}
