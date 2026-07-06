package subscription

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSendConfirmationCallsNotificationService(t *testing.T) {
	var got struct {
		Email      string `json:"email"`
		Repo       string `json:"repo"`
		ConfirmURL string `json:"confirm_url"`
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/notifications/confirmation" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	publisher := NewOutboxNotificationPublisher(nil, "http://api.local", server.URL)
	if err := publisher.SendConfirmation(context.Background(), "USER@example.com", "owner/repo", "token-1"); err != nil {
		t.Fatalf("SendConfirmation() error = %v", err)
	}
	if got.Email != "USER@example.com" || got.Repo != "owner/repo" || got.ConfirmURL != "http://api.local/api/confirm/token-1" {
		t.Fatalf("request = %#v", got)
	}
}

func TestBuildReleaseNotification(t *testing.T) {
	event, raw, err := buildReleaseNotification("user@example.com", "owner/repo", "v2.0.0", "notification-id", "corr-1", time.Now().UTC())
	if err != nil {
		t.Fatalf("buildReleaseNotification() error = %v", err)
	}
	if event.Payload.IdempotencyKey != "owner/repo:v2.0.0:"+event.Payload.EmailHash {
		t.Fatalf("idempotency key = %q", event.Payload.IdempotencyKey)
	}
	if event.Payload.Message != "New release v2.0.0 for owner/repo!" {
		t.Fatalf("message = %q", event.Payload.Message)
	}
	if event.Payload.Tag != "v2.0.0" || event.Payload.Repo != "owner/repo" {
		t.Fatalf("payload = %#v", event.Payload)
	}
	if !json.Valid(raw) {
		t.Fatal("raw payload must be valid JSON")
	}
}
