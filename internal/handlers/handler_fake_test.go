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
	"github.com/stretchr/testify/mock"

	"subber/internal/models"
	"subber/internal/service"
)

type mockSubscriptionRepository struct {
	mock.Mock
}

func (m *mockSubscriptionRepository) ConfirmSubscriptionByToken(ctx context.Context, token string) error {
	return m.Called(ctx, token).Error(0)
}

func (m *mockSubscriptionRepository) Unsubscribe(ctx context.Context, token string) error {
	return m.Called(ctx, token).Error(0)
}

func (m *mockSubscriptionRepository) GetSubscriptions(ctx context.Context, email string) ([]models.Subscription, error) {
	args := m.Called(ctx, email)
	return args.Get(0).([]models.Subscription), args.Error(1)
}

type mockSubscriptionService struct {
	mock.Mock
}

func (m *mockSubscriptionService) Subscribe(ctx context.Context, email, repo string) error {
	return m.Called(ctx, email, repo).Error(0)
}

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
	svc := new(mockSubscriptionService)
	svc.On("Subscribe", mock.Anything, "a@b.com", "owner/repo").Return(service.ErrAlreadySubscribed).Once()
	r := newRouter(new(mockSubscriptionRepository), svc)

	req := httptest.NewRequest(http.MethodPost, "/subscribe", subscribeBody(t, "a@b.com", "owner/repo"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	svc.AssertExpectations(t)

	if w.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d", w.Code)
	}
}

func TestSubscribeHandler_RepoNotFound_Returns404(t *testing.T) {
	svc := new(mockSubscriptionService)
	svc.On("Subscribe", mock.Anything, "a@b.com", "owner/repo").Return(service.ErrRepoNotFound).Once()
	r := newRouter(new(mockSubscriptionRepository), svc)

	req := httptest.NewRequest(http.MethodPost, "/subscribe", subscribeBody(t, "a@b.com", "owner/repo"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	svc.AssertExpectations(t)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestSubscribeHandler_RateLimit_Returns429(t *testing.T) {
	// Rate limit must be explicit — 429 tells the client to back off, not retry immediately.
	svc := new(mockSubscriptionService)
	svc.On("Subscribe", mock.Anything, "a@b.com", "owner/repo").Return(service.ErrGitHubRateLimit).Once()
	r := newRouter(new(mockSubscriptionRepository), svc)

	req := httptest.NewRequest(http.MethodPost, "/subscribe", subscribeBody(t, "a@b.com", "owner/repo"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	svc.AssertExpectations(t)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429, got %d", w.Code)
	}
}

func TestSubscribeHandler_GitHubUnavailable_Returns502(t *testing.T) {
	// Upstream failures must be 502 Bad Gateway, not 500 — distinguishes our fault from GitHub's.
	svc := new(mockSubscriptionService)
	svc.On("Subscribe", mock.Anything, "a@b.com", "owner/repo").Return(service.ErrGitHubUnavailable).Once()
	r := newRouter(new(mockSubscriptionRepository), svc)

	req := httptest.NewRequest(http.MethodPost, "/subscribe", subscribeBody(t, "a@b.com", "owner/repo"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	svc.AssertExpectations(t)

	if w.Code != http.StatusBadGateway {
		t.Errorf("expected 502, got %d", w.Code)
	}
}

func TestSubscribeHandler_UnknownError_Returns500(t *testing.T) {
	svc := new(mockSubscriptionService)
	svc.On("Subscribe", mock.Anything, "a@b.com", "owner/repo").Return(errors.New("unexpected")).Once()
	r := newRouter(new(mockSubscriptionRepository), svc)

	req := httptest.NewRequest(http.MethodPost, "/subscribe", subscribeBody(t, "a@b.com", "owner/repo"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	svc.AssertExpectations(t)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestSubscribeHandler_Success_Returns200(t *testing.T) {
	svc := new(mockSubscriptionService)
	svc.On("Subscribe", mock.Anything, "a@b.com", "owner/repo").Return(nil).Once()
	r := newRouter(new(mockSubscriptionRepository), svc)

	req := httptest.NewRequest(http.MethodPost, "/subscribe", subscribeBody(t, "a@b.com", "owner/repo"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	svc.AssertExpectations(t)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

// — ConfirmByToken handler —

func TestConfirmByToken_Success_Returns200(t *testing.T) {
	token := uuid.New().String()
	repo := new(mockSubscriptionRepository)
	repo.On("ConfirmSubscriptionByToken", mock.Anything, token).Return(nil).Once()
	r := newRouter(repo, new(mockSubscriptionService))

	req := httptest.NewRequest(http.MethodGet, "/confirm/"+token, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	repo.AssertExpectations(t)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestConfirmByToken_UnknownToken_Returns404(t *testing.T) {
	// Unknown token must return 404 so the user knows the link is stale or invalid.
	token := uuid.New().String()
	repo := new(mockSubscriptionRepository)
	repo.On("ConfirmSubscriptionByToken", mock.Anything, token).Return(errors.New("not found")).Once()
	r := newRouter(repo, new(mockSubscriptionService))

	req := httptest.NewRequest(http.MethodGet, "/confirm/"+token, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	repo.AssertExpectations(t)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

// — UnsubscribeByToken handler —

func TestUnsubscribeByToken_Success_Returns200(t *testing.T) {
	token := uuid.New().String()
	repo := new(mockSubscriptionRepository)
	repo.On("Unsubscribe", mock.Anything, token).Return(nil).Once()
	r := newRouter(repo, new(mockSubscriptionService))

	req := httptest.NewRequest(http.MethodGet, "/unsubscribe/"+token, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	repo.AssertExpectations(t)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestUnsubscribeByToken_UnknownToken_Returns404(t *testing.T) {
	token := uuid.New().String()
	repo := new(mockSubscriptionRepository)
	repo.On("Unsubscribe", mock.Anything, token).Return(errors.New("not found")).Once()
	r := newRouter(repo, new(mockSubscriptionService))

	req := httptest.NewRequest(http.MethodGet, "/unsubscribe/"+token, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	repo.AssertExpectations(t)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

// — GetSubscriptions handler —

func TestGetSubscriptions_ReturnsSubscriptionList(t *testing.T) {
	subs := []models.Subscription{
		{Email: "a@b.com", Repo: "owner/repo", Confirmed: true},
	}
	repo := new(mockSubscriptionRepository)
	repo.On("GetSubscriptions", mock.Anything, "a@b.com").Return(subs, nil).Once()
	r := newRouter(repo, new(mockSubscriptionService))

	req := httptest.NewRequest(http.MethodGet, "/subscriptions/?email=a@b.com", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	repo.AssertExpectations(t)

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
	repo := new(mockSubscriptionRepository)
	repo.On("GetSubscriptions", mock.Anything, "a@b.com").
		Return([]models.Subscription(nil), errors.New("db error")).
		Once()
	r := newRouter(repo, new(mockSubscriptionService))

	req := httptest.NewRequest(http.MethodGet, "/subscriptions/?email=a@b.com", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	repo.AssertExpectations(t)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}
