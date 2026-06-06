//go:build integration

package integration

import (
	"net/http"
	"testing"
)

func TestConfirm_Success(t *testing.T) {
	env := newTestEnv(t, newGitHubMock(http.StatusOK))

	w := env.subscribe(t, "user@example.com", "owner/repo")
	if w.Code != http.StatusOK {
		t.Fatalf("subscribe: status = %d", w.Code)
	}

	token := tokenForSubscription(t, env.pool, "user@example.com", "owner/repo")
	w = env.confirm(t, token)

	if w.Code != http.StatusOK {
		t.Errorf("confirm: status = %d, want 200, body: %s", w.Code, w.Body.String())
	}
	if !isConfirmed(t, env.pool, "user@example.com", "owner/repo") {
		t.Error("subscription must be confirmed in DB after confirm")
	}
}

func TestConfirm_UnknownToken_Returns404(t *testing.T) {
	env := newTestEnv(t, newGitHubMock(http.StatusOK))

	w := env.confirm(t, "00000000-0000-0000-0000-000000000000")

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}
