package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"subber/infra/cache"
	"subber/models"
)

var GitHubAPIBase = "https://api.github.com"

func setGitHubAPIBase(base string) {
	GitHubAPIBase = base
}

var httpClient = &http.Client{Timeout: 10 * time.Second}

func GetLatestTag(ctx context.Context, repo string, token string, rc *cache.RedisCache) (string, error) {
	cacheKey := "github:latest_tag:" + repo

	if rc != nil {
		cached, err := rc.Get(ctx, cacheKey)
		if err == nil && cached != "" {
			return cached, nil
		}
	}

	link := fmt.Sprintf("%s/repos/%s/releases/latest", GitHubAPIBase, repo)

	req, err := http.NewRequestWithContext(ctx, "GET", link, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Go-Subber-App")

	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return "", nil
	}

	if resp.StatusCode == http.StatusTooManyRequests {
		return "", fmt.Errorf("github rate limit exceeded (429)")
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("github error: %d", resp.StatusCode)
	}

	var release models.GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", err
	}

	if rc != nil && release.LastSeenTag != "" {
		_ = rc.Set(ctx, cacheKey, release.LastSeenTag, 10*time.Minute)
	}

	return release.LastSeenTag, nil
}

func CheckIfRepoExists(ctx context.Context, repo string, token string) (*http.Response, error) {
	link := fmt.Sprintf("%s/repos/%s", GitHubAPIBase, repo)

	req, err := http.NewRequestWithContext(ctx, "HEAD", link, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "Go-Subber-App")

	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	return httpClient.Do(req)
}
