package subscription

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPGitHubClient_GetLatestTag(t *testing.T) {
	tests := []struct {
		name string
		code int
		body string
		want string
		err  error
	}{
		{"success", http.StatusOK, `{"tag_name":"v1.2.3"}`, "v1.2.3", nil},
		{"not found", http.StatusNotFound, ``, "", nil},
		{"rate limit", http.StatusTooManyRequests, ``, "", ErrGitHubAPILimit},
		{"server error", http.StatusInternalServerError, ``, "", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.code)
				if tt.body != "" {
					_, _ = w.Write([]byte(tt.body))
				}
			}))
			defer server.Close()

			tag, err := NewHTTPGitHubClient(server.URL, "").GetLatestTag(context.Background(), "owner/repo")
			if tt.err != nil && !errors.Is(err, tt.err) {
				t.Fatalf("error = %v, want %v", err, tt.err)
			}
			if tt.err == nil && tt.code >= 500 && err == nil {
				t.Fatal("expected error for server status, got nil")
			}
			if tag != tt.want {
				t.Fatalf("tag = %q, want %q", tag, tt.want)
			}
		})
	}
}

func TestHTTPGitHubClient_CheckIfRepoExists(t *testing.T) {
	tests := []struct {
		name string
		code int
		err  error
	}{
		{"success", http.StatusOK, nil},
		{"not found", http.StatusNotFound, ErrGitHubNotFound},
		{"rate limit", http.StatusTooManyRequests, ErrGitHubAPILimit},
		{"server error", http.StatusInternalServerError, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodHead {
					t.Fatalf("method = %s, want HEAD", r.Method)
				}
				w.WriteHeader(tt.code)
			}))
			defer server.Close()

			err := NewHTTPGitHubClient(server.URL, "").CheckIfRepoExists(context.Background(), "owner/repo")
			if tt.err != nil && !errors.Is(err, tt.err) {
				t.Fatalf("error = %v, want %v", err, tt.err)
			}
			if tt.err == nil && tt.code >= 500 && err == nil {
				t.Fatal("expected error for server status, got nil")
			}
		})
	}
}

func TestHTTPGitHubClient_SendsAuthHeader(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte(`{"tag_name":"v2.0.0"}`))
	}))
	defer server.Close()

	tag, err := NewHTTPGitHubClient(server.URL, "test-token").GetLatestTag(context.Background(), "owner/repo")
	if err != nil {
		t.Fatalf("GetLatestTag() error = %v", err)
	}
	if tag != "v2.0.0" {
		t.Fatalf("tag = %q, want v2.0.0", tag)
	}
}
