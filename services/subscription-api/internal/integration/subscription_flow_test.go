//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"reflect"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/jackc/pgx/v5"

	"subber/pkg/contracts"
	"subber/services/subscription-api/internal/subscription"
)

func TestSubscribe_SuccessPersistsUnconfirmedAndWritesConfirmationOutbox(t *testing.T) {
	env := newTestEnv(t, newGitHubMock(t, nil, "v1.0.0", nil))

	res := env.subscribe(t, "user@example.com", "owner/repo", "test-key")

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body: %s", res.Code, res.Body.String())
	}
	if !subscriptionExists(t, "user@example.com", "owner/repo") {
		t.Fatal("subscription not saved")
	}
	if isConfirmed(t, "user@example.com", "owner/repo") {
		t.Fatal("subscription must start unconfirmed")
	}
	if notificationCommandCount(t) != 1 {
		t.Fatalf("notification commands = %d, want 1", notificationCommandCount(t))
	}
}

func TestSaveSubscriptionWithConfirmation_RollsBackWhenOutboxFails(t *testing.T) {
	env := newTestEnv(t, newGitHubMock(t, nil, "", nil))
	repo := subscription.NewRepository(env.pool)
	ctrl := gomock.NewController(t)
	publisher := NewMockConfirmationPublisher(ctrl)
	outboxErr := errors.New("outbox down")
	publisher.EXPECT().
		PublishConfirmationTx(gomock.Any(), gomock.Any(), "rollback@example.com", "owner/repo", "00000000-0000-0000-0000-000000000001").
		Return(outboxErr)

	err := repo.SaveSubscriptionWithConfirmationRequest(
		context.Background(),
		subscription.Subscription{
			Email:       "rollback@example.com",
			Repo:        "owner/repo",
			LastSeenTag: "v1.0.0",
			Token:       "00000000-0000-0000-0000-000000000001",
		},
		publisher,
	)
	if err == nil {
		t.Fatal("expected outbox error, got nil")
	}
	if subscriptionExists(t, "rollback@example.com", "owner/repo") {
		t.Fatal("subscription must roll back when confirmation outbox insert fails")
	}
	if notificationCommandCount(t) != 0 {
		t.Fatalf("notification commands = %d, want 0", notificationCommandCount(t))
	}
}

func TestSubscribe_DuplicateAndRepoValidation(t *testing.T) {
	env := newTestEnv(t, newGitHubMock(t, nil, "", nil))

	env.subscribe(t, "user@example.com", "owner/repo", "test-key")
	res := env.subscribe(t, "user@example.com", "owner/repo", "test-key")
	if res.Code != http.StatusConflict {
		t.Fatalf("duplicate status = %d, want 409", res.Code)
	}

	env = newTestEnv(t, newGitHubMock(t, githubError(http.StatusNotFound), "", nil))
	res = env.subscribe(t, "user@example.com", "owner/missing", "test-key")
	if res.Code != http.StatusNotFound {
		t.Fatalf("missing repo status = %d, want 404", res.Code)
	}
	if subscriptionExists(t, "user@example.com", "owner/missing") {
		t.Fatal("subscription must not be saved when GitHub rejects repo")
	}
}

type MockConfirmationPublisher struct {
	ctrl     *gomock.Controller
	recorder *MockConfirmationPublisherMockRecorder
}

type MockConfirmationPublisherMockRecorder struct {
	mock *MockConfirmationPublisher
}

func NewMockConfirmationPublisher(ctrl *gomock.Controller) *MockConfirmationPublisher {
	mock := &MockConfirmationPublisher{ctrl: ctrl}
	mock.recorder = &MockConfirmationPublisherMockRecorder{mock}
	return mock
}

func (m *MockConfirmationPublisher) EXPECT() *MockConfirmationPublisherMockRecorder {
	return m.recorder
}

func (m *MockConfirmationPublisher) PublishConfirmationTx(ctx context.Context, tx pgx.Tx, email, repo, token string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "PublishConfirmationTx", ctx, tx, email, repo, token)
	ret0, _ := ret[0].(error)
	return ret0
}

func (mr *MockConfirmationPublisherMockRecorder) PublishConfirmationTx(ctx, tx, email, repo, token interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "PublishConfirmationTx", reflect.TypeOf((*MockConfirmationPublisher)(nil).PublishConfirmationTx), ctx, tx, email, repo, token)
}

func TestSubscribe_WithoutAPIKeyReturnsUnauthorized(t *testing.T) {
	env := newTestEnv(t, newGitHubMock(t, nil, "", nil))

	res := env.subscribe(t, "user@example.com", "owner/repo", "")

	if res.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", res.Code)
	}
}

func TestConfirm_FirstSubscriberWritesRepoWatchSagaStart(t *testing.T) {
	env := newTestEnv(t, newGitHubMock(t, nil, "", nil))
	env.subscribe(t, "user@example.com", "owner/repo", "test-key")
	token := tokenForSubscription(t, "user@example.com", "owner/repo")

	res := env.confirm(t, token)

	if res.Code != http.StatusOK {
		t.Fatalf("confirm status = %d, want 200", res.Code)
	}
	if !isConfirmed(t, "user@example.com", "owner/repo") {
		t.Fatal("subscription must be confirmed")
	}
	if outboxCount(t, contracts.EventRepoWatchSagaRequested, contracts.TopicWatchlistSagaRequests) != 1 {
		t.Fatalf("repo watch saga requests = %d, want 1", outboxCount(t, contracts.EventRepoWatchSagaRequested, contracts.TopicWatchlistSagaRequests))
	}
}

func TestConfirm_SecondSubscriberDoesNotWriteDuplicateRepoWatchSagaStart(t *testing.T) {
	env := newTestEnv(t, newGitHubMock(t, nil, "", nil))
	env.subscribe(t, "a@example.com", "owner/repo", "test-key")
	env.confirm(t, tokenForSubscription(t, "a@example.com", "owner/repo"))
	env.subscribe(t, "b@example.com", "owner/repo", "test-key")

	res := env.confirm(t, tokenForSubscription(t, "b@example.com", "owner/repo"))

	if res.Code != http.StatusOK {
		t.Fatalf("confirm status = %d, want 200", res.Code)
	}
	if outboxCount(t, contracts.EventRepoWatchSagaRequested, contracts.TopicWatchlistSagaRequests) != 1 {
		t.Fatalf("repo watch saga requests = %d, want only first subscriber event", outboxCount(t, contracts.EventRepoWatchSagaRequested, contracts.TopicWatchlistSagaRequests))
	}
}

func TestConfirm_UnknownTokenReturns404(t *testing.T) {
	env := newTestEnv(t, newGitHubMock(t, nil, "", nil))

	res := env.confirm(t, "00000000-0000-0000-0000-000000000000")

	if res.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", res.Code)
	}
}

func TestUnsubscribe_LastSubscriberWritesRepoWatchSagaStop(t *testing.T) {
	env := newTestEnv(t, newGitHubMock(t, nil, "", nil))
	env.subscribe(t, "user@example.com", "owner/repo", "test-key")
	token := tokenForSubscription(t, "user@example.com", "owner/repo")
	env.confirm(t, token)

	res := env.unsubscribe(t, token)

	if res.Code != http.StatusOK {
		t.Fatalf("unsubscribe status = %d, want 200", res.Code)
	}
	if subscriptionExists(t, "user@example.com", "owner/repo") {
		t.Fatal("subscription must be deleted")
	}
	if outboxCount(t, contracts.EventRepoWatchSagaRequested, contracts.TopicWatchlistSagaRequests) != 2 {
		t.Fatalf("repo watch saga requests = %d, want 2 start+stop", outboxCount(t, contracts.EventRepoWatchSagaRequested, contracts.TopicWatchlistSagaRequests))
	}
}

func TestUnsubscribe_NotLastSubscriberDoesNotWriteRepoWatchStop(t *testing.T) {
	env := newTestEnv(t, newGitHubMock(t, nil, "", nil))
	env.subscribe(t, "a@example.com", "owner/repo", "test-key")
	tokenA := tokenForSubscription(t, "a@example.com", "owner/repo")
	env.confirm(t, tokenA)
	env.subscribe(t, "b@example.com", "owner/repo", "test-key")
	tokenB := tokenForSubscription(t, "b@example.com", "owner/repo")
	env.confirm(t, tokenB)

	res := env.unsubscribe(t, tokenA)

	if res.Code != http.StatusOK {
		t.Fatalf("unsubscribe status = %d, want 200", res.Code)
	}
	if outboxCount(t, contracts.EventRepoWatchSagaRequested, contracts.TopicWatchlistSagaRequests) != 1 {
		t.Fatalf("repo watch saga requests = %d, want only initial start while another subscriber remains", outboxCount(t, contracts.EventRepoWatchSagaRequested, contracts.TopicWatchlistSagaRequests))
	}
}

func TestGetSubscriptions_ReturnsConfirmedOnlyAndRequiresAPIKey(t *testing.T) {
	env := newTestEnv(t, newGitHubMock(t, nil, "", nil))
	env.subscribe(t, "user@example.com", "owner/repo1", "test-key")
	env.confirm(t, tokenForSubscription(t, "user@example.com", "owner/repo1"))
	env.subscribe(t, "user@example.com", "owner/repo2", "test-key")

	res := env.getSubscriptions(t, "user@example.com", "test-key")

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", res.Code)
	}
	var subs []map[string]any
	if err := json.NewDecoder(res.Body).Decode(&subs); err != nil {
		t.Fatalf("decode subscriptions: %v", err)
	}
	if len(subs) != 1 || subs[0]["repo"] != "owner/repo1" {
		t.Fatalf("subscriptions = %#v, want only confirmed owner/repo1", subs)
	}

	res = env.getSubscriptions(t, "user@example.com", "")
	if res.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized status = %d, want 401", res.Code)
	}
}
