package httpapi

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

	"subber/pkg/requestid"
	"subber/services/subscription-api/internal/subscription"
)

type fakeSubscriptionReader struct {
	confirmCalled bool
	unsubCalled   bool
	subscriptions []subscription.Subscription
	err           error
}

func (r *fakeSubscriptionReader) GetSubscriptions(context.Context, string) ([]subscription.Subscription, error) {
	return r.subscriptions, r.err
}

func (r *fakeSubscriptionReader) ConfirmSubscriptionByToken(context.Context, string) error {
	r.confirmCalled = true
	return r.err
}

func (r *fakeSubscriptionReader) Unsubscribe(context.Context, string) error {
	r.unsubCalled = true
	return r.err
}

type fakeSubscriptionCreator struct {
	called bool
	err    error
}

func (c *fakeSubscriptionCreator) Subscribe(context.Context, string, string) error {
	c.called = true
	return c.err
}

func newTestRouter(apiKey string, reader *fakeSubscriptionReader, creator *fakeSubscriptionCreator) http.Handler {
	gin.SetMode(gin.TestMode)
	return SetupRouter(RouterDeps{
		APIKey:  apiKey,
		Repo:    reader,
		Service: creator,
	})
}

func TestSubscribe_RequiresAPIKey(t *testing.T) {
	creator := &fakeSubscriptionCreator{}
	router := newTestRouter("secret", &fakeSubscriptionReader{}, creator)

	req := httptest.NewRequest(http.MethodPost, "/api/subscribe", bytes.NewBufferString(`{"email":"user@example.com","repo":"owner/repo"}`))
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)

	if res.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", res.Code)
	}
	if creator.called {
		t.Fatal("service should not be called without API key")
	}
}

func TestSubscribe_RejectsInvalidRepoBeforeService(t *testing.T) {
	creator := &fakeSubscriptionCreator{}
	router := newTestRouter("secret", &fakeSubscriptionReader{}, creator)

	req := httptest.NewRequest(http.MethodPost, "/api/subscribe", bytes.NewBufferString(`{"email":"user@example.com","repo":"broken"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "secret")
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", res.Code)
	}
	if creator.called {
		t.Fatal("service should not be called for invalid repo")
	}
}

func TestSubscribe_MapsServiceErrorsToHTTPStatus(t *testing.T) {
	body := `{"email":"user@example.com","repo":"owner/repo"}`
	tests := []struct {
		name string
		err  error
		want int
	}{
		{"already subscribed", subscription.ErrAlreadySubscribed, http.StatusConflict},
		{"repo not found", subscription.ErrRepoNotFound, http.StatusNotFound},
		{"rate limit", subscription.ErrGitHubRateLimit, http.StatusTooManyRequests},
		{"github unavailable", subscription.ErrGitHubUnavailable, http.StatusBadGateway},
		{"unknown", errors.New("boom"), http.StatusInternalServerError},
		{"success", nil, http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := newTestRouter("", &fakeSubscriptionReader{}, &fakeSubscriptionCreator{err: tt.err})
			req := httptest.NewRequest(http.MethodPost, "/api/subscribe", bytes.NewBufferString(body))
			req.Header.Set("Content-Type", "application/json")
			res := httptest.NewRecorder()
			router.ServeHTTP(res, req)
			if res.Code != tt.want {
				t.Fatalf("status = %d, want %d", res.Code, tt.want)
			}
		})
	}
}

func TestSubscribe_RejectsInvalidJSON(t *testing.T) {
	creator := &fakeSubscriptionCreator{}
	router := newTestRouter("", &fakeSubscriptionReader{}, creator)

	req := httptest.NewRequest(http.MethodPost, "/api/subscribe", bytes.NewBufferString(`not json`))
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", res.Code)
	}
	if creator.called {
		t.Fatal("service should not be called for invalid JSON")
	}
}

func TestGetSubscriptions_RejectsInvalidEmail(t *testing.T) {
	router := newTestRouter("secret", &fakeSubscriptionReader{}, &fakeSubscriptionCreator{})

	req := httptest.NewRequest(http.MethodGet, "/api/subscriptions/?email=bad-email", nil)
	req.Header.Set("X-API-Key", "secret")
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", res.Code)
	}
}

func TestGetSubscriptions_ReturnsSubscriptionsAndRepositoryErrors(t *testing.T) {
	tests := []struct {
		name string
		repo *fakeSubscriptionReader
		want int
	}{
		{
			name: "success",
			repo: &fakeSubscriptionReader{subscriptions: []subscription.Subscription{
				{Email: "user@example.com", Repo: "owner/repo", Confirmed: true},
			}},
			want: http.StatusOK,
		},
		{
			name: "repository error",
			repo: &fakeSubscriptionReader{err: errors.New("db down")},
			want: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := newTestRouter("", tt.repo, &fakeSubscriptionCreator{})
			req := httptest.NewRequest(http.MethodGet, "/api/subscriptions/?email=user@example.com", nil)
			res := httptest.NewRecorder()
			router.ServeHTTP(res, req)

			if res.Code != tt.want {
				t.Fatalf("status = %d, want %d", res.Code, tt.want)
			}
			if tt.want == http.StatusOK {
				var got []subscription.Subscription
				if err := json.Unmarshal(res.Body.Bytes(), &got); err != nil {
					t.Fatalf("decode response: %v", err)
				}
				if len(got) != 1 || got[0].Repo != "owner/repo" {
					t.Fatalf("subscriptions = %#v", got)
				}
			}
		})
	}
}

func TestConfirm_RejectsInvalidTokenBeforeRepository(t *testing.T) {
	reader := &fakeSubscriptionReader{}
	router := newTestRouter("", reader, &fakeSubscriptionCreator{})

	req := httptest.NewRequest(http.MethodGet, "/api/confirm/not-a-uuid", nil)
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", res.Code)
	}
	if reader.confirmCalled {
		t.Fatal("repository should not be called for invalid token")
	}
}

func TestConfirm_MapsRepositoryResult(t *testing.T) {
	token := uuid.NewString()
	tests := []struct {
		name string
		err  error
		want int
	}{
		{"success", nil, http.StatusOK},
		{"unknown token", errors.New("not found"), http.StatusNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := &fakeSubscriptionReader{err: tt.err}
			router := newTestRouter("", reader, &fakeSubscriptionCreator{})
			req := httptest.NewRequest(http.MethodGet, "/api/confirm/"+token, nil)
			res := httptest.NewRecorder()
			router.ServeHTTP(res, req)

			if res.Code != tt.want {
				t.Fatalf("status = %d, want %d", res.Code, tt.want)
			}
			if !reader.confirmCalled {
				t.Fatal("repository should be called for valid token")
			}
		})
	}
}

func TestUnsubscribe_MapsRepositoryResult(t *testing.T) {
	token := uuid.NewString()
	tests := []struct {
		name string
		err  error
		want int
	}{
		{"success", nil, http.StatusOK},
		{"unknown token", errors.New("not found"), http.StatusNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := &fakeSubscriptionReader{err: tt.err}
			router := newTestRouter("", reader, &fakeSubscriptionCreator{})
			req := httptest.NewRequest(http.MethodGet, "/api/unsubscribe/"+token, nil)
			res := httptest.NewRecorder()
			router.ServeHTTP(res, req)

			if res.Code != tt.want {
				t.Fatalf("status = %d, want %d", res.Code, tt.want)
			}
			if !reader.unsubCalled {
				t.Fatal("repository should be called for valid token")
			}
		})
	}
}

func TestRequestIDMiddleware_PropagatesValidHeader(t *testing.T) {
	router := newTestRouter("", &fakeSubscriptionReader{}, &fakeSubscriptionCreator{})
	req := httptest.NewRequest(http.MethodGet, "/api/subscriptions/?email=user@example.com", nil)
	req.Header.Set(requestid.Header, "client-request-1")
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)

	if got := res.Header().Get(requestid.Header); got != "client-request-1" {
		t.Fatalf("response request id = %q, want client-request-1", got)
	}
}

func TestRequestIDMiddleware_GeneratesMissingHeader(t *testing.T) {
	router := newTestRouter("", &fakeSubscriptionReader{}, &fakeSubscriptionCreator{})
	req := httptest.NewRequest(http.MethodGet, "/api/subscriptions/?email=user@example.com", nil)
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)

	if got := res.Header().Get(requestid.Header); got == "" {
		t.Fatal("request id missing from response header")
	}
}
