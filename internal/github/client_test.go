package github

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

type mockRoundTripper func(*http.Request) (*http.Response, error)

func (m mockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return m(req)
}

func newMockClient(status int, body string) *Client {
	return &Client{
		baseURL: "https://api.github.test",
		httpClient: &http.Client{Transport: mockRoundTripper(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: status,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(body)),
				Request:    req,
			}, nil
		})},
	}
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
			tag, err := newMockClient(tt.status, tt.body).GetLatestTag(context.Background(), "owner/repo")

			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr = %v", err, tt.wantErr)
			}
			if tag != tt.want {
				t.Errorf("tag = %q, want %q", tag, tt.want)
			}
		})
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
			err := newMockClient(tt.status, "").CheckIfRepoExists(context.Background(), "owner/repo")

			if !errors.Is(err, tt.wantErr) {
				t.Errorf("err = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestGetLatestTag_SendsAuthHeader(t *testing.T) {
	c := &Client{
		baseURL: "https://api.github.test",
		httpClient: &http.Client{Transport: mockRoundTripper(func(req *http.Request) (*http.Response, error) {
			if req.Header.Get("Authorization") != "Bearer test-token" {
				return &http.Response{
					StatusCode: http.StatusUnauthorized,
					Body:       io.NopCloser(strings.NewReader("")),
					Request:    req,
				}, nil
			}

			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{"tag_name":"v2.0.0"}`)),
				Request:    req,
			}, nil
		})},
		token: "test-token",
	}

	tag, err := c.GetLatestTag(context.Background(), "owner/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tag != "v2.0.0" {
		t.Errorf("tag = %q, want v2.0.0", tag)
	}
}
