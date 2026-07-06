//go:build integration

package integration

import (
	"encoding/json"
	"net/http"
	"testing"

	"subber/pkg/contracts"
)

func TestSubscribe_SuccessPersistsUnconfirmedAndSendsConfirmationRequest(t *testing.T) {
	env := newTestEnv(t, newGitHubMock(t, nil, "v1.0.0", nil), http.StatusAccepted)

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
	if env.confirmationRequestCount() != 1 {
		t.Fatalf("confirmation requests = %d, want 1", env.confirmationRequestCount())
	}
	request := env.latestConfirmationRequest(t)
	if request.Email != "user@example.com" || request.Repo != "owner/repo" {
		t.Fatalf("confirmation request = %#v", request)
	}
	if want := "http://localhost:8080/api/confirm/" + tokenForSubscription(t, "user@example.com", "owner/repo"); request.ConfirmURL != want {
		t.Fatalf("confirm url = %q, want %q", request.ConfirmURL, want)
	}
}

func TestSubscribe_RollsBackWhenConfirmationFails(t *testing.T) {
	env := newTestEnv(t, newGitHubMock(t, nil, "", nil), http.StatusServiceUnavailable)

	res := env.subscribe(t, "rollback@example.com", "owner/repo", "test-key")

	if res.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503, body: %s", res.Code, res.Body.String())
	}
	if subscriptionExists(t, "rollback@example.com", "owner/repo") {
		t.Fatal("subscription must roll back when confirmation request fails")
	}
	if env.confirmationRequestCount() != 1 {
		t.Fatalf("confirmation requests = %d, want 1", env.confirmationRequestCount())
	}
}

func TestSubscribe_DuplicateAndRepoValidation(t *testing.T) {
	env := newTestEnv(t, newGitHubMock(t, nil, "", nil), http.StatusAccepted)

	env.subscribe(t, "user@example.com", "owner/repo", "test-key")
	res := env.subscribe(t, "user@example.com", "owner/repo", "test-key")
	if res.Code != http.StatusConflict {
		t.Fatalf("duplicate status = %d, want 409", res.Code)
	}

	env = newTestEnv(t, newGitHubMock(t, githubError(http.StatusNotFound), "", nil), http.StatusAccepted)
	res = env.subscribe(t, "user@example.com", "owner/missing", "test-key")
	if res.Code != http.StatusNotFound {
		t.Fatalf("missing repo status = %d, want 404", res.Code)
	}
	if subscriptionExists(t, "user@example.com", "owner/missing") {
		t.Fatal("subscription must not be saved when GitHub rejects repo")
	}
}

func TestSubscribe_WithoutAPIKeyReturnsUnauthorized(t *testing.T) {
	env := newTestEnv(t, newGitHubMock(t, nil, "", nil), http.StatusAccepted)

	res := env.subscribe(t, "user@example.com", "owner/repo", "")

	if res.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", res.Code)
	}
}

func TestConfirm_FirstSubscriberWritesRepoWatchSagaStart(t *testing.T) {
	env := newTestEnv(t, newGitHubMock(t, nil, "", nil), http.StatusAccepted)
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
	env := newTestEnv(t, newGitHubMock(t, nil, "", nil), http.StatusAccepted)
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
	env := newTestEnv(t, newGitHubMock(t, nil, "", nil), http.StatusAccepted)

	res := env.confirm(t, "00000000-0000-0000-0000-000000000000")

	if res.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", res.Code)
	}
}

func TestUnsubscribe_LastSubscriberWritesRepoWatchSagaStop(t *testing.T) {
	env := newTestEnv(t, newGitHubMock(t, nil, "", nil), http.StatusAccepted)
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
	env := newTestEnv(t, newGitHubMock(t, nil, "", nil), http.StatusAccepted)
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
	env := newTestEnv(t, newGitHubMock(t, nil, "", nil), http.StatusAccepted)
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
