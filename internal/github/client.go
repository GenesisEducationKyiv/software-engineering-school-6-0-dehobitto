package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"subber/internal/infra/cache"
	"subber/internal/logger"
	"subber/internal/models"
)

var ghLog = logger.New().WithField("component", "github")

const (
	defaultBaseURL = "https://api.github.com"
	userAgent      = "Go-Subber-App"
	cacheKeyPrefix = "github:latest_tag:"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
	token      string
	cache      cache.Cache
}

func NewClient(token string, c cache.Cache) *Client {
	return &Client{
		baseURL:    defaultBaseURL,
		httpClient: &http.Client{Timeout: 10 * time.Second},
		token:      token,
		cache:      c,
	}
}

func (c *Client) GetLatestTag(ctx context.Context, repo string) (string, error) {
	cacheKey := cacheKeyPrefix + repo

	if c.cache != nil {
		cached, err := c.cache.Get(ctx, cacheKey)
		if err != nil {
			ghLog.WithField("repo", repo).WithError(err).Warn("cache get failed")
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
	req.Header.Set("User-Agent", userAgent)
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	start := time.Now()
	resp, err := c.httpClient.Do(req)
	duration := time.Since(start).Milliseconds()
	if err != nil {
		ghLog.WithField("repo", repo).WithField("duration_ms", duration).WithField("status", "unavailable").WithError(err).Error("github api call failed")
		return "", err
	}
	defer resp.Body.Close() //nolint:errcheck

	switch resp.StatusCode {
	case http.StatusNotFound:
		ghLog.WithField("repo", repo).WithField("duration_ms", duration).WithField("status", "not_found").Info("github api call")
		return "", nil
	case http.StatusTooManyRequests:
		ghLog.WithField("repo", repo).WithField("duration_ms", duration).WithField("status", "rate_limited").Warn("github api rate limited")
		return "", fmt.Errorf("github rate limit exceeded (429)")
	case http.StatusOK:
		ghLog.WithField("repo", repo).WithField("duration_ms", duration).WithField("status", "ok").Info("github api call")
	default:
		ghLog.WithField("repo", repo).WithField("duration_ms", duration).WithField("status", "unavailable").WithField("http_code", resp.StatusCode).Error("github api unexpected status")
		return "", fmt.Errorf("github error: %d", resp.StatusCode)
	}

	var release models.GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", err
	}

	if c.cache != nil && release.LastSeenTag != "" {
		if err := c.cache.Set(ctx, cacheKey, release.LastSeenTag, 45*time.Second); err != nil {
			ghLog.WithField("repo", repo).WithError(err).Warn("failed to cache tag")
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

	req.Header.Set("User-Agent", userAgent)
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
