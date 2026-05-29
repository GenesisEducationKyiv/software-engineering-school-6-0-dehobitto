package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"subber/internal/models"
	"subber/internal/service"
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
type fakeSvc struct{ err error }

func (f *fakeSvc) Subscribe(_ context.Context, _, _ string) error { return f.err }

func newRouter(repo SubscriptionRepository, svc SubscriptionService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewHandler(repo, svc)
	r.POST("/subscribe", h.Subscribe)
	r.GET("/confirm/:token", h.ConfirmByToken)
	r.GET("/unsubscribe/:token", h.UnsubscribeByToken)
	r.GET("/subscriptions/", h.GetSubscriptions)
	return r
}

func subscribeBody(t *testing.T, email, repo string) *bytes.Buffer {
	t.Helper()
	b, _ := json.Marshal(map[string]string{"email": email, "repo": repo})
	return bytes.NewBuffer(b)
}

// — Subscribe handler —

func TestSubscribeHandler_AlreadySubscribed_Returns409(t *testing.T) {
	// Duplicate subscription must return 409 Conflict, not 500, so clients can handle it gracefully.
	r := newRouter(&fakeHandlerRepo{}, &fakeSvc{err: service.ErrAlreadySubscribed})

	req := httptest.NewRequest(http.MethodPost, "/subscribe", subscribeBody(t, "a@b.com", "owner/repo"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d", w.Code)
	}
}

func TestSubscribeHandler_RepoNotFound_Returns404(t *testing.T) {
	r := newRouter(&fakeHandlerRepo{}, &fakeSvc{err: service.ErrRepoNotFound})

	req := httptest.NewRequest(http.MethodPost, "/subscribe", subscribeBody(t, "a@b.com", "owner/repo"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestSubscribeHandler_RateLimit_Returns429(t *testing.T) {
	// Rate limit must be explicit — 429 tells the client to back off, not retry immediately.
	r := newRouter(&fakeHandlerRepo{}, &fakeSvc{err: service.ErrGitHubRateLimit})

	req := httptest.NewRequest(http.MethodPost, "/subscribe", subscribeBody(t, "a@b.com", "owner/repo"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429, got %d", w.Code)
	}
}

func TestSubscribeHandler_GitHubUnavailable_Returns502(t *testing.T) {
	// Upstream failures must be 502 Bad Gateway, not 500 — distinguishes our fault from GitHub's.
	r := newRouter(&fakeHandlerRepo{}, &fakeSvc{err: service.ErrGitHubUnavailable})

	req := httptest.NewRequest(http.MethodPost, "/subscribe", subscribeBody(t, "a@b.com", "owner/repo"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadGateway {
		t.Errorf("expected 502, got %d", w.Code)
	}
}

func TestSubscribeHandler_UnknownError_Returns500(t *testing.T) {
	r := newRouter(&fakeHandlerRepo{}, &fakeSvc{err: errors.New("unexpected")})

	req := httptest.NewRequest(http.MethodPost, "/subscribe", subscribeBody(t, "a@b.com", "owner/repo"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestSubscribeHandler_Success_Returns200(t *testing.T) {
	r := newRouter(&fakeHandlerRepo{}, &fakeSvc{})

	req := httptest.NewRequest(http.MethodPost, "/subscribe", subscribeBody(t, "a@b.com", "owner/repo"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

// — ConfirmByToken handler —

func TestConfirmByToken_Success_Returns200(t *testing.T) {
	r := newRouter(&fakeHandlerRepo{}, &fakeSvc{})
	token := uuid.New().String()

	req := httptest.NewRequest(http.MethodGet, "/confirm/"+token, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestConfirmByToken_UnknownToken_Returns404(t *testing.T) {
	// Unknown token must return 404 so the user knows the link is stale or invalid.
	r := newRouter(&fakeHandlerRepo{confirmErr: errors.New("not found")}, &fakeSvc{})
	token := uuid.New().String()

	req := httptest.NewRequest(http.MethodGet, "/confirm/"+token, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

// — UnsubscribeByToken handler —

func TestUnsubscribeByToken_Success_Returns200(t *testing.T) {
	r := newRouter(&fakeHandlerRepo{}, &fakeSvc{})
	token := uuid.New().String()

	req := httptest.NewRequest(http.MethodGet, "/unsubscribe/"+token, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestUnsubscribeByToken_UnknownToken_Returns404(t *testing.T) {
	r := newRouter(&fakeHandlerRepo{unsubErr: errors.New("not found")}, &fakeSvc{})
	token := uuid.New().String()

	req := httptest.NewRequest(http.MethodGet, "/unsubscribe/"+token, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

// — GetSubscriptions handler —

func TestGetSubscriptions_ReturnsSubscriptionList(t *testing.T) {
	subs := []models.Subscription{
		{Email: "a@b.com", Repo: "owner/repo", Confirmed: true},
	}
	r := newRouter(&fakeHandlerRepo{subs: subs}, &fakeSvc{})

	req := httptest.NewRequest(http.MethodGet, "/subscriptions/?email=a@b.com", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var got []models.Subscription
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("could not decode response: %v", err)
	}
	if len(got) != 1 || got[0].Repo != "owner/repo" {
		t.Errorf("unexpected response body: %v", got)
	}
}

func TestGetSubscriptions_RepoError_Returns500(t *testing.T) {
	r := newRouter(&fakeHandlerRepo{subsErr: errors.New("db error")}, &fakeSvc{})

	req := httptest.NewRequest(http.MethodGet, "/subscriptions/?email=a@b.com", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}
