package github

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"subber/internal/infra/cache"
	"subber/internal/models"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
	token      string
	cache      cache.Cache
}

func NewClient(token string, c cache.Cache) *Client {
	return &Client{
		baseURL:    "https://api.github.com",
		httpClient: &http.Client{Timeout: 10 * time.Second},
		token:      token,
		cache:      c,
	}
}

func (c *Client) GetLatestTag(ctx context.Context, repo string) (string, error) {
	cacheKey := "github:latest_tag:" + repo

	if c.cache != nil {
		cached, err := c.cache.Get(ctx, cacheKey)
		if err != nil {
			log.Printf("cache get failed for %s: %v", repo, err)
		}
		if cached != "" {
			return cached, nil
		}
	}

	link := fmt.Sprintf("%s/repos/%s/releases/latest", c.baseURL, repo)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, link, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Go-Subber-App")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
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

	if c.cache != nil && release.LastSeenTag != "" {
		if err := c.cache.Set(ctx, cacheKey, release.LastSeenTag, 45*time.Second); err != nil {
			log.Printf("failed to cache tag for %s: %v", repo, err)
		}
	}

	return release.LastSeenTag, nil
}

func (c *Client) CheckIfRepoExists(ctx context.Context, repo string) error {
	link := fmt.Sprintf("%s/repos/%s", c.baseURL, repo)

	req, err := http.NewRequestWithContext(ctx, http.MethodHead, link, nil)
	if err != nil {
		return err
	}

	req.Header.Set("User-Agent", "Go-Subber-App")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close() //nolint:errcheck

	switch resp.StatusCode {
	case http.StatusOK:
		return nil
	case http.StatusNotFound:
		return ErrNotFound
	case http.StatusTooManyRequests:
		return ErrRateLimit
	default:
		return fmt.Errorf("github error: %d", resp.StatusCode)
	}
}
