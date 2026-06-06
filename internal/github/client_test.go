package github

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/mock"
)

type mockRoundTripper struct {
	mock.Mock
}

func (m *mockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	args := m.Called(req)
	return args.Get(0).(*http.Response), args.Error(1)
}

func newMockClient(status int, body string) *Client {
	transport := new(mockRoundTripper)
	transport.On("RoundTrip", mock.Anything).Return(response(status, body), nil)

	return &Client{
		baseURL:    "https://api.github.test",
		httpClient: &http.Client{Transport: transport},
	}
}

func response(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
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
	_, err := newMockClient(http.StatusTooManyRequests, "").GetLatestTag(context.Background(), "owner/repo")
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
	transport := new(mockRoundTripper)
	transport.On("RoundTrip", mock.MatchedBy(func(req *http.Request) bool {
		return req.Header.Get("Authorization") == "Bearer test-token"
	})).Return(response(http.StatusOK, `{"tag_name":"v2.0.0"}`), nil).Once()

	c := &Client{
		baseURL:    "https://api.github.test",
		httpClient: &http.Client{Transport: transport},
		token:      "test-token",
	}

	tag, err := c.GetLatestTag(context.Background(), "owner/repo")
	transport.AssertExpectations(t)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tag != "v2.0.0" {
		t.Errorf("tag = %q, want v2.0.0", tag)
	}
}
