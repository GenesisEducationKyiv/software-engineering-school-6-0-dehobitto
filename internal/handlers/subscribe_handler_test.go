package handlers

import (
	"errors"
	"net/http"
	"testing"

	"subber/internal/service"
)

func TestSubscribe_InputValidation(t *testing.T) {
	r := newTestRouter(&fakeHandlerRepo{}, &fakeSvc{})

	tests := []struct {
		name string
		body []byte
		want int
	}{
		{"invalid json", []byte("not json"), http.StatusBadRequest},
		{"invalid repo format", jsonBody(t, map[string]string{"email": "a@b.com", "repo": "noslash"}), http.StatusBadRequest},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := do(r, http.MethodPost, "/subscribe", tt.body)
			if w.Code != tt.want {
				t.Errorf("status = %d, want %d", w.Code, tt.want)
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
