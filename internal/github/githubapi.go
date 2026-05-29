package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"subber/internal/infra/cache"
	"subber/internal/models"
)

// GitHubClient holds the HTTP client and base URL so both are injectable in tests.
type GitHubClient struct {
	baseURL    string
	httpClient *http.Client
}

func NewGitHubClient() *GitHubClient {
	return &GitHubClient{
		baseURL:    "https://api.github.com",
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *GitHubClient) GetLatestTag(ctx context.Context, repo, token string, rc cache.Cache) (string, error) {
	cacheKey := "github:latest_tag:" + repo

	if rc != nil {
		if cached, err := rc.Get(ctx, cacheKey); err == nil && cached != "" {
			return cached, nil
		}
	}

	link := fmt.Sprintf("%s/repos/%s/releases/latest", c.baseURL, repo)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, link, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Go-Subber-App")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close() //nolint:errcheck

	switch resp.StatusCode {
	case http.StatusNotFound:
		return "", nil
	case http.StatusTooManyRequests:
		return "", fmt.Errorf("github rate limit exceeded (429)")
	case http.StatusOK:
	default:
		return "", fmt.Errorf("github error: %d", resp.StatusCode)
	}

	var release models.GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", err
	}

	if rc != nil && release.LastSeenTag != "" {
		_ = rc.Set(ctx, cacheKey, release.LastSeenTag, 45*time.Second)
	}

	return release.LastSeenTag, nil
}

func (c *GitHubClient) CheckIfRepoExists(ctx context.Context, repo, token string) (*http.Response, error) {
	link := fmt.Sprintf("%s/repos/%s", c.baseURL, repo)

	req, err := http.NewRequestWithContext(ctx, http.MethodHead, link, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "Go-Subber-App")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	return c.httpClient.Do(req)
}
