package github

import (
	"context"
	"testing"

	"net/http"
	"net/http/httptest"
)

func newTestClient(url string) *GitHubClient {
	return &GitHubClient{
		baseURL:    url,
		httpClient: &http.Client{},
	}
}

func TestGetLatestTag_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"tag_name": "v1.2.3"}`))
	}))
	defer server.Close()

	tag, err := newTestClient(server.URL).GetLatestTag(context.Background(), "owner/repo", "", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if tag != "v1.2.3" {
		t.Errorf("expected v1.2.3, got %s", tag)
	}
}

func TestGetLatestTag_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	tag, err := newTestClient(server.URL).GetLatestTag(context.Background(), "owner/repo", "", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tag != "" {
		t.Errorf("expected empty tag, got %s", tag)
	}
}

func TestGetLatestTag_RateLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	_, err := newTestClient(server.URL).GetLatestTag(context.Background(), "owner/repo", "", nil)
	if err == nil {
		t.Fatal("expected error for 429, got nil")
	}
}

func TestCheckIfRepoExists_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	resp, err := newTestClient(server.URL).CheckIfRepoExists(context.Background(), "owner/repo", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestCheckIfRepoExists_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	resp, err := newTestClient(server.URL).CheckIfRepoExists(context.Background(), "owner/nonexistent", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

func TestGetLatestTag_AuthHeader(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"tag_name": "v2.0.0"}`))
	}))
	defer server.Close()

	tag, err := newTestClient(server.URL).GetLatestTag(context.Background(), "owner/repo", "test-token", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tag != "v2.0.0" {
		t.Errorf("expected v2.0.0, got %s", tag)
	}
}
