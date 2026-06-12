package github

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPReleaseProvider_GetLatestTag(t *testing.T) {
	tests := []struct {
		name    string
		code    int
		body    string
		wantTag string
		wantErr bool
	}{
		{"success", http.StatusOK, `{"tag_name":"v1.2.3"}`, "v1.2.3", false},
		{"not found", http.StatusNotFound, ``, "", false},
		{"rate limit", http.StatusTooManyRequests, ``, "", true},
		{"server error", http.StatusInternalServerError, ``, "", true},
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

			tag, err := NewHTTPReleaseProvider(server.URL, "").GetLatestTag(context.Background(), "owner/repo")
			if tt.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("GetLatestTag() error = %v", err)
			}
			if tag != tt.wantTag {
				t.Fatalf("tag = %q, want %q", tag, tt.wantTag)
			}
		})
	}
}

func TestHTTPReleaseProvider_SendsAuthHeader(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte(`{"tag_name":"v2.0.0"}`))
	}))
	defer server.Close()

	tag, err := NewHTTPReleaseProvider(server.URL, "test-token").GetLatestTag(context.Background(), "owner/repo")
	if err != nil {
		t.Fatalf("GetLatestTag() error = %v", err)
	}
	if tag != "v2.0.0" {
		t.Fatalf("tag = %q, want v2.0.0", tag)
	}
}
