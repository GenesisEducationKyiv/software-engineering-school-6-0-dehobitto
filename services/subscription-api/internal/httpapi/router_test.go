package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/golang/mock/gomock"
	"github.com/google/uuid"

	"subber/pkg/requestid"
	"subber/services/subscription-api/internal/subscription"
)

type MockSubscriptionReader struct {
	ctrl     *gomock.Controller
	recorder *MockSubscriptionReaderMockRecorder
}

type MockSubscriptionReaderMockRecorder struct {
	mock *MockSubscriptionReader
}

func NewMockSubscriptionReader(ctrl *gomock.Controller) *MockSubscriptionReader {
	mock := &MockSubscriptionReader{ctrl: ctrl}
	mock.recorder = &MockSubscriptionReaderMockRecorder{mock}
	return mock
}

func (m *MockSubscriptionReader) EXPECT() *MockSubscriptionReaderMockRecorder {
	return m.recorder
}

func (m *MockSubscriptionReader) ConfirmSubscriptionByToken(ctx context.Context, token string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ConfirmSubscriptionByToken", ctx, token)
	ret0, _ := ret[0].(error)
	return ret0
}

func (mr *MockSubscriptionReaderMockRecorder) ConfirmSubscriptionByToken(ctx, token interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ConfirmSubscriptionByToken", reflect.TypeOf((*MockSubscriptionReader)(nil).ConfirmSubscriptionByToken), ctx, token)
}

func (m *MockSubscriptionReader) GetSubscriptions(ctx context.Context, email string) ([]subscription.Subscription, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetSubscriptions", ctx, email)
	ret0, _ := ret[0].([]subscription.Subscription)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

func (mr *MockSubscriptionReaderMockRecorder) GetSubscriptions(ctx, email interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetSubscriptions", reflect.TypeOf((*MockSubscriptionReader)(nil).GetSubscriptions), ctx, email)
}

func (m *MockSubscriptionReader) Unsubscribe(ctx context.Context, token string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Unsubscribe", ctx, token)
	ret0, _ := ret[0].(error)
	return ret0
}

func (mr *MockSubscriptionReaderMockRecorder) Unsubscribe(ctx, token interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Unsubscribe", reflect.TypeOf((*MockSubscriptionReader)(nil).Unsubscribe), ctx, token)
}

type MockSubscriptionCreator struct {
	ctrl     *gomock.Controller
	recorder *MockSubscriptionCreatorMockRecorder
}

type MockSubscriptionCreatorMockRecorder struct {
	mock *MockSubscriptionCreator
}

func NewMockSubscriptionCreator(ctrl *gomock.Controller) *MockSubscriptionCreator {
	mock := &MockSubscriptionCreator{ctrl: ctrl}
	mock.recorder = &MockSubscriptionCreatorMockRecorder{mock}
	return mock
}

func (m *MockSubscriptionCreator) EXPECT() *MockSubscriptionCreatorMockRecorder {
	return m.recorder
}

func (m *MockSubscriptionCreator) Subscribe(ctx context.Context, email, repo string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Subscribe", ctx, email, repo)
	ret0, _ := ret[0].(error)
	return ret0
}

func (mr *MockSubscriptionCreatorMockRecorder) Subscribe(ctx, email, repo interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Subscribe", reflect.TypeOf((*MockSubscriptionCreator)(nil).Subscribe), ctx, email, repo)
}

func newTestRouter(apiKey string, reader SubscriptionReader, creator SubscriptionCreator) http.Handler {
	gin.SetMode(gin.TestMode)
	return SetupRouter(RouterDeps{
		APIKey:  apiKey,
		Repo:    reader,
		Service: creator,
	})
}

func TestSubscribe_RequiresAPIKey(t *testing.T) {
	ctrl := gomock.NewController(t)
	router := newTestRouter("secret", NewMockSubscriptionReader(ctrl), NewMockSubscriptionCreator(ctrl))

	req := httptest.NewRequest(http.MethodPost, "/api/subscribe", bytes.NewBufferString(`{"email":"user@example.com","repo":"owner/repo"}`))
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)

	if res.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", res.Code)
	}
}

func TestSubscribe_RejectsInvalidRepoBeforeService(t *testing.T) {
	ctrl := gomock.NewController(t)
	router := newTestRouter("secret", NewMockSubscriptionReader(ctrl), NewMockSubscriptionCreator(ctrl))

	req := httptest.NewRequest(http.MethodPost, "/api/subscribe", bytes.NewBufferString(`{"email":"user@example.com","repo":"broken"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "secret")
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", res.Code)
	}
}

func TestSubscribe_RejectsInvalidEmailBeforeService(t *testing.T) {
	ctrl := gomock.NewController(t)
	router := newTestRouter("secret", NewMockSubscriptionReader(ctrl), NewMockSubscriptionCreator(ctrl))

	req := httptest.NewRequest(http.MethodPost, "/api/subscribe", bytes.NewBufferString(`{"email":"bad-email","repo":"owner/repo"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "secret")
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", res.Code)
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
			ctrl := gomock.NewController(t)
			creator := NewMockSubscriptionCreator(ctrl)
			creator.EXPECT().Subscribe(gomock.Any(), "user@example.com", "owner/repo").Return(tt.err)
			router := newTestRouter("", NewMockSubscriptionReader(ctrl), creator)
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
	ctrl := gomock.NewController(t)
	router := newTestRouter("", NewMockSubscriptionReader(ctrl), NewMockSubscriptionCreator(ctrl))

	req := httptest.NewRequest(http.MethodPost, "/api/subscribe", bytes.NewBufferString(`not json`))
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", res.Code)
	}
}

func TestGetSubscriptions_RejectsInvalidEmail(t *testing.T) {
	ctrl := gomock.NewController(t)
	router := newTestRouter("secret", NewMockSubscriptionReader(ctrl), NewMockSubscriptionCreator(ctrl))

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
		name          string
		subscriptions []subscription.Subscription
		err           error
		want          int
	}{
		{
			name: "success",
			subscriptions: []subscription.Subscription{
				{Email: "user@example.com", Repo: "owner/repo", Confirmed: true},
			},
			want: http.StatusOK,
		},
		{
			name: "repository error",
			err:  errors.New("db down"),
			want: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			reader := NewMockSubscriptionReader(ctrl)
			reader.EXPECT().GetSubscriptions(gomock.Any(), "user@example.com").Return(tt.subscriptions, tt.err)
			router := newTestRouter("", reader, NewMockSubscriptionCreator(ctrl))
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
	ctrl := gomock.NewController(t)
	router := newTestRouter("", NewMockSubscriptionReader(ctrl), NewMockSubscriptionCreator(ctrl))

	req := httptest.NewRequest(http.MethodGet, "/api/confirm/not-a-uuid", nil)
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", res.Code)
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
			ctrl := gomock.NewController(t)
			reader := NewMockSubscriptionReader(ctrl)
			reader.EXPECT().ConfirmSubscriptionByToken(gomock.Any(), token).Return(tt.err)
			router := newTestRouter("", reader, NewMockSubscriptionCreator(ctrl))
			req := httptest.NewRequest(http.MethodGet, "/api/confirm/"+token, nil)
			res := httptest.NewRecorder()
			router.ServeHTTP(res, req)

			if res.Code != tt.want {
				t.Fatalf("status = %d, want %d", res.Code, tt.want)
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
			ctrl := gomock.NewController(t)
			reader := NewMockSubscriptionReader(ctrl)
			reader.EXPECT().Unsubscribe(gomock.Any(), token).Return(tt.err)
			router := newTestRouter("", reader, NewMockSubscriptionCreator(ctrl))
			req := httptest.NewRequest(http.MethodGet, "/api/unsubscribe/"+token, nil)
			res := httptest.NewRecorder()
			router.ServeHTTP(res, req)

			if res.Code != tt.want {
				t.Fatalf("status = %d, want %d", res.Code, tt.want)
			}
		})
	}
}

func TestRequestIDMiddleware_PropagatesValidHeader(t *testing.T) {
	ctrl := gomock.NewController(t)
	reader := NewMockSubscriptionReader(ctrl)
	reader.EXPECT().GetSubscriptions(gomock.Any(), "user@example.com").Return(nil, nil)
	router := newTestRouter("", reader, NewMockSubscriptionCreator(ctrl))
	req := httptest.NewRequest(http.MethodGet, "/api/subscriptions/?email=user@example.com", nil)
	req.Header.Set(requestid.Header, "client-request-1")
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)

	if got := res.Header().Get(requestid.Header); got != "client-request-1" {
		t.Fatalf("response request id = %q, want client-request-1", got)
	}
}

func TestRequestIDMiddleware_GeneratesMissingHeader(t *testing.T) {
	ctrl := gomock.NewController(t)
	reader := NewMockSubscriptionReader(ctrl)
	reader.EXPECT().GetSubscriptions(gomock.Any(), "user@example.com").Return(nil, nil)
	router := newTestRouter("", reader, NewMockSubscriptionCreator(ctrl))
	req := httptest.NewRequest(http.MethodGet, "/api/subscriptions/?email=user@example.com", nil)
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)

	if got := res.Header().Get(requestid.Header); got == "" {
		t.Fatal("request id missing from response header")
	}
}
