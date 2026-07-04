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
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"subber/pkg/requestid"
	"subber/pkg/logger"
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

func newObservedTestRouter(apiKey string, reader *fakeSubscriptionReader, creator *fakeSubscriptionCreator, log logger.Logger, metrics *AccessMetrics) http.Handler {
	gin.SetMode(gin.TestMode)
	registry := prometheus.NewRegistry()
	if metrics == nil {
		metrics = NewAccessMetrics()
	}
	registry.MustRegister(metrics.Requests, metrics.Duration)
	return SetupRouter(RouterDeps{
		APIKey:        apiKey,
		Repo:          reader,
		Service:       creator,
		Logger:        log,
		Gatherer:      registry,
		AccessMetrics: metrics,
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

func TestSubscribe_RejectsRequestsWhenConfiguredAPIKeyIsEmpty(t *testing.T) {
	creator := &fakeSubscriptionCreator{}
	router := newTestRouter("", &fakeSubscriptionReader{}, creator)

	req := httptest.NewRequest(http.MethodPost, "/api/subscribe", bytes.NewBufferString(`{"email":"user@example.com","repo":"owner/repo"}`))
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)

	if res.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", res.Code)
	}
	if creator.called {
		t.Fatal("service should not be called when API key config is empty")
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

func TestSubscribe_RejectsInvalidEmailBeforeService(t *testing.T) {
	creator := &fakeSubscriptionCreator{}
	router := newTestRouter("secret", &fakeSubscriptionReader{}, creator)

	req := httptest.NewRequest(http.MethodPost, "/api/subscribe", bytes.NewBufferString(`{"email":"bad-email","repo":"owner/repo"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "secret")
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", res.Code)
	}
	if creator.called {
		t.Fatal("service should not be called for invalid email")
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
			router := newTestRouter("secret", &fakeSubscriptionReader{}, &fakeSubscriptionCreator{err: tt.err})
			req := httptest.NewRequest(http.MethodPost, "/api/subscribe", bytes.NewBufferString(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-API-Key", "secret")
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
	router := newTestRouter("secret", &fakeSubscriptionReader{}, creator)

	req := httptest.NewRequest(http.MethodPost, "/api/subscribe", bytes.NewBufferString(`not json`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "secret")
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
			router := newTestRouter("secret", tt.repo, &fakeSubscriptionCreator{})
			req := httptest.NewRequest(http.MethodGet, "/api/subscriptions/?email=user@example.com", nil)
			req.Header.Set("X-API-Key", "secret")
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
	router := newTestRouter("secret", &fakeSubscriptionReader{}, &fakeSubscriptionCreator{})
	req := httptest.NewRequest(http.MethodGet, "/api/subscriptions/?email=user@example.com", nil)
	req.Header.Set(requestid.Header, "client-request-1")
	req.Header.Set("X-API-Key", "secret")
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)

	if got := res.Header().Get(requestid.Header); got != "client-request-1" {
		t.Fatalf("response request id = %q, want client-request-1", got)
	}
}

func TestRequestIDMiddleware_GeneratesMissingHeader(t *testing.T) {
	router := newTestRouter("secret", &fakeSubscriptionReader{}, &fakeSubscriptionCreator{})
	req := httptest.NewRequest(http.MethodGet, "/api/subscriptions/?email=user@example.com", nil)
	req.Header.Set("X-API-Key", "secret")
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)

	if got := res.Header().Get(requestid.Header); got == "" {
		t.Fatal("request id missing from response header")
	}
}

func TestAPIAccessMiddleware_LogsAndCountsAPIRequestsOnly(t *testing.T) {
	metrics := NewAccessMetrics()
	log := newFakeLogger()
	router := newObservedTestRouter("secret", &fakeSubscriptionReader{}, &fakeSubscriptionCreator{}, log, metrics)

	req := httptest.NewRequest(http.MethodGet, "/api/subscriptions/?email=bad-email", nil)
	req.Header.Set("X-API-Key", "secret")
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", res.Code)
	}

	if len(*log.entries) != 1 {
		t.Fatalf("access log entries = %d, want 1", len(*log.entries))
	}
	got := (*log.entries)[0]
	if got.fields["component"] != "access" {
		t.Fatalf("component = %#v, want access", got.fields["component"])
	}
	if got.fields["route"] != "/api/subscriptions/" {
		t.Fatalf("route = %#v, want /api/subscriptions/", got.fields["route"])
	}
	if got.fields["status"] != 400 {
		t.Fatalf("status = %#v, want 400", got.fields["status"])
	}
	if got.fields["request_id"] == "" {
		t.Fatal("request_id missing from access log")
	}
	if want := 1.0; testutil.ToFloat64(metrics.Requests.WithLabelValues("GET", "/api/subscriptions/", "400")) != want {
		t.Fatalf("request counter = %v, want %v", testutil.ToFloat64(metrics.Requests.WithLabelValues("GET", "/api/subscriptions/", "400")), want)
	}

	metricsReq := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	metricsRes := httptest.NewRecorder()
	router.ServeHTTP(metricsRes, metricsReq)
	if metricsRes.Code != http.StatusOK {
		t.Fatalf("metrics status = %d, want 200", metricsRes.Code)
	}
	if len(*log.entries) != 1 {
		t.Fatalf("access log entries after /metrics = %d, want 1", len(*log.entries))
	}
	if got := testutil.ToFloat64(metrics.Requests.WithLabelValues("GET", "/api/subscriptions/", "400")); got != 1 {
		t.Fatalf("request counter after /metrics = %v, want 1", got)
	}
}

type fakeLogger struct {
	entries *[]loggedEntry
	fields  map[string]any
}

type loggedEntry struct {
	level  string
	msg    string
	fields map[string]any
}

func newFakeLogger() *fakeLogger {
	entries := make([]loggedEntry, 0, 8)
	return &fakeLogger{entries: &entries, fields: map[string]any{}}
}

func (l *fakeLogger) WithField(key string, value any) logger.Logger {
	next := &fakeLogger{
		entries: l.entries,
		fields:  cloneFields(l.fields),
	}
	next.fields[key] = value
	return next
}

func (l *fakeLogger) WithError(err error) logger.Logger {
	if err == nil {
		return l
	}
	return l.WithField("error", err.Error())
}

func (l *fakeLogger) Info(msg string)  { l.append("info", msg) }
func (l *fakeLogger) Warn(msg string)  { l.append("warn", msg) }
func (l *fakeLogger) Error(msg string) { l.append("error", msg) }
func (l *fakeLogger) Fatal(msg string)  { l.append("fatal", msg) }

func (l *fakeLogger) append(level, msg string) {
	entry := loggedEntry{
		level:  level,
		msg:    msg,
		fields: cloneFields(l.fields),
	}
	*l.entries = append(*l.entries, entry)
}

func cloneFields(src map[string]any) map[string]any {
	dst := make(map[string]any, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}
