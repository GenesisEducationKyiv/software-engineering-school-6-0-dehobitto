package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/gin-gonic/gin"
)

type fakeSvc struct{ err error }

func (f *fakeSvc) Subscribe(_ context.Context, _, _ string) error { return f.err }

func newSubscribeRouter(svc subscriptionService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/api/subscribe", (&Handler{svc: svc}).Subscribe)
	return r
}

func newTokenRouter(h *Handler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/api/confirm/:token", h.ConfirmByToken)
	r.GET("/api/unsubscribe/:token", h.UnsubscribeByToken)
	return r
}

func subscribeBody(t *testing.T, email, repo string) *bytes.Buffer {
	t.Helper()
	b, _ := json.Marshal(map[string]string{"email": email, "repo": repo})
	return bytes.NewBuffer(b)
}
