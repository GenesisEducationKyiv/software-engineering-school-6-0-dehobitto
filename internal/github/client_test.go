package github

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newTestClient(url string) *Client {
	return &Client{baseURL: url, httpClient: &http.Client{}}
}

func serve(status int, body string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if body != "" {
			w.Header().Set("Content-Type", "application/json")
		}
		w.WriteHeader(status)
		if body != "" {
			w.Write([]byte(body)) //nolint:errcheck
		}
	}))
}

func TestGetLatestTag(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		body    string
		want    string
		wantErr bool
	}{
		{"200 returns tag", http.StatusOK, `{"tag_name":"v1.2.3"}`, "v1.2.3", false},
		{"404 returns empty tag", http.StatusNotFound, "", "", false},
		{"429 returns error", http.StatusTooManyRequests, "", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := serve(tt.status, tt.body)
			defer s.Close()

			tag, err := newTestClient(s.URL).GetLatestTag(context.Background(), "owner/repo")

			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr = %v", err, tt.wantErr)
			}
			if tag != tt.want {
				t.Errorf("tag = %q, want %q", tag, tt.want)
			}
		})
	}
}

func TestCheckIfRepoExists(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		wantErr error
	}{
		{"200 → nil", http.StatusOK, nil},
		{"404 → ErrNotFound", http.StatusNotFound, ErrNotFound},
		{"429 → ErrRateLimit", http.StatusTooManyRequests, ErrRateLimit},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := serve(tt.status, "")
			defer s.Close()

			err := newTestClient(s.URL).CheckIfRepoExists(context.Background(), "owner/repo")

			if !errors.Is(err, tt.wantErr) {
				t.Errorf("err = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestGetLatestTag_SendsAuthHeader(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"tag_name":"v2.0.0"}`)) //nolint:errcheck
	}))
	defer s.Close()

	c := &Client{baseURL: s.URL, httpClient: &http.Client{}, token: "test-token"}
	tag, err := c.GetLatestTag(context.Background(), "owner/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tag != "v2.0.0" {
		t.Errorf("tag = %q, want v2.0.0", tag)
	}
}
