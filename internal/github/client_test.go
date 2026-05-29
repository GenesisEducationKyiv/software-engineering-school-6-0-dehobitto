package github

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newTestClient(url string) *Client {
	return &Client{
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

	tag, err := newTestClient(server.URL).GetLatestTag(context.Background(), "owner/repo")
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

	tag, err := newTestClient(server.URL).GetLatestTag(context.Background(), "owner/repo")
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

	_, err := newTestClient(server.URL).GetLatestTag(context.Background(), "owner/repo")
	if err == nil {
		t.Fatal("expected error for 429, got nil")
	}
}

func TestCheckIfRepoExists_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	if err := newTestClient(server.URL).CheckIfRepoExists(context.Background(), "owner/repo"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCheckIfRepoExists_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	err := newTestClient(server.URL).CheckIfRepoExists(context.Background(), "owner/nonexistent")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
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

	c := newTestClient(server.URL)
	c.token = "test-token"

	tag, err := c.GetLatestTag(context.Background(), "owner/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tag != "v2.0.0" {
		t.Errorf("expected v2.0.0, got %s", tag)
	}
}
