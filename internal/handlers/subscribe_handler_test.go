package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"subber/internal/service"
)

func TestSubscribe_InputValidation(t *testing.T) {
	tests := []struct {
		name      string
		body      []byte
		wantError string
	}{
		{"invalid json", []byte("not json"), "Invalid JSON"},
		{"invalid repo format", jsonBody(t, map[string]string{"email": "a@b.com", "repo": "noslash"}), "Invalid repository format"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &fakeSvc{}
			r := newTestRouter(&fakeHandlerRepo{}, svc)

			w := do(r, http.MethodPost, "/subscribe", tt.body)
			if w.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
			}

			var got map[string]string
			if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
				t.Fatalf("response json: %v", err)
			}
			if got["error"] != tt.wantError {
				t.Errorf("error = %q, want %q", got["error"], tt.wantError)
			}
			if svc.calls != 0 {
				t.Errorf("service calls = %d, want 0", svc.calls)
			}
		})
	}
}

func TestSubscribe_ServiceErrorToStatus(t *testing.T) {
	body := jsonBody(t, map[string]string{"email": "a@b.com", "repo": "owner/repo"})

	tests := []struct {
		name string
		err  error
		want int
	}{
		// 409: client can detect duplicate and show a friendly message
		{"already subscribed", service.ErrAlreadySubscribed, http.StatusConflict},
		// 404: repo doesn't exist on GitHub
		{"repo not found", service.ErrRepoNotFound, http.StatusNotFound},
		// 429: GitHub is rate-limiting us; client should back off
		{"rate limited", service.ErrGitHubRateLimit, http.StatusTooManyRequests},
		// 502: upstream GitHub is down, not our fault
		{"github unavailable", service.ErrGitHubUnavailable, http.StatusBadGateway},
		// 500: unexpected internal error
		{"unknown error", errors.New("boom"), http.StatusInternalServerError},
		// 200: happy path
		{"success", nil, http.StatusOK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newTestRouter(&fakeHandlerRepo{}, &fakeSvc{err: tt.err})
			w := do(r, http.MethodPost, "/subscribe", body)
			if w.Code != tt.want {
				t.Errorf("status = %d, want %d", w.Code, tt.want)
			}
		})
	}
}
