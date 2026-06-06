//go:build integration

package integration

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestGetSubscriptions_ReturnsConfirmedOnly(t *testing.T) {
	env := newTestEnv(t, gitHubFake{repoStatus: http.StatusOK})

	// subscribe and confirm first repo
	env.subscribe(t, "user@example.com", "owner/repo1")
	token := tokenForSubscription(t, env.pool, "user@example.com", "owner/repo1")
	env.confirm(t, token)

	// subscribe but do not confirm second repo
	env.subscribe(t, "user@example.com", "owner/repo2")

	w := env.getSubscriptions(t, "user@example.com", "test-key")

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	var subs []map[string]any
	if err := json.NewDecoder(w.Body).Decode(&subs); err != nil {
		t.Fatalf("decode json: %v", err)
	}
	if len(subs) != 1 {
		t.Errorf("subscriptions = %d, want 1 (only confirmed)", len(subs))
	}
}

func TestGetSubscriptions_ReturnsEmptyList(t *testing.T) {
	env := newTestEnv(t, gitHubFake{repoStatus: http.StatusOK})

	w := env.getSubscriptions(t, "nobody@example.com", "test-key")

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	var subs []map[string]any
	if err := json.NewDecoder(w.Body).Decode(&subs); err != nil {
		t.Fatalf("decode json: %v", err)
	}
	if len(subs) != 0 {
		t.Errorf("subscriptions = %d, want 0", len(subs))
	}
}

func TestGetSubscriptions_WithoutAPIKey_ReturnsUnauthorized(t *testing.T) {
	env := newTestEnv(t, gitHubFake{repoStatus: http.StatusOK})

	w := env.getSubscriptions(t, "user@example.com", "")

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}
