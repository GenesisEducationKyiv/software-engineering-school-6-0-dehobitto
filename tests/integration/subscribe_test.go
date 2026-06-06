//go:build integration

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSubscribe_Success(t *testing.T) {
	env := newTestEnv(t, gitHubFake{repoStatus: http.StatusOK})

	w := env.subscribe(t, "user@example.com", "owner/repo")

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body: %s", w.Code, w.Body.String())
	}
	if !subscriptionExists(t, env.pool, "user@example.com", "owner/repo") {
		t.Error("subscription not saved in DB")
	}
	if isConfirmed(t, env.pool, "user@example.com", "owner/repo") {
		t.Error("subscription must not be confirmed before email confirmation")
	}
}

func TestSubscribe_Duplicate_ReturnsConflict(t *testing.T) {
	env := newTestEnv(t, gitHubFake{repoStatus: http.StatusOK})

	env.subscribe(t, "user@example.com", "owner/repo")
	w := env.subscribe(t, "user@example.com", "owner/repo")

	if w.Code != http.StatusConflict {
		t.Errorf("status = %d, want 409", w.Code)
	}

	var count int
	env.pool.QueryRow(context.Background(),
		"SELECT COUNT(*) FROM subscriptions WHERE email=$1 AND repo=$2",
		"user@example.com", "owner/repo",
	).Scan(&count)
	if count != 1 {
		t.Errorf("rows in DB = %d, want 1", count)
	}
}

func TestSubscribe_RepoNotFound_Returns404(t *testing.T) {
	env := newTestEnv(t, gitHubFake{repoStatus: http.StatusNotFound})

	w := env.subscribe(t, "user@example.com", "owner/nonexistent")

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
	if subscriptionExists(t, env.pool, "user@example.com", "owner/nonexistent") {
		t.Error("subscription must not be saved when repo not found")
	}
}

func TestSubscribe_WithoutAPIKey_ReturnsUnauthorized(t *testing.T) {
	env := newTestEnv(t, gitHubFake{repoStatus: http.StatusOK})

	body, err := json.Marshal(map[string]string{"email": "user@example.com", "repo": "owner/repo"})
	if err != nil {
		t.Fatalf("encode json: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/subscribe", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	env.router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}
