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

var GitHubAPIBase = "https://api.github.com"

func setGitHubAPIBase(base string) {
	GitHubAPIBase = base
}

var httpClient = &http.Client{Timeout: 10 * time.Second}

func GetLatestTag(ctx context.Context, repo, token string, cache cache.Cache) (string, error) {
	cacheKey := "github:latest_tag:" + repo

	if cache != nil {
		cached, err := cache.Get(ctx, cacheKey)
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
	defer resp.Body.Close()

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

	if cache != nil && release.LastSeenTag != "" {
		if err := cache.Set(ctx, cacheKey, release.LastSeenTag, 10*time.Minute); err != nil {
			log.Printf("failed to cache tag for %s: %v", cacheKey, err)
		}
	}

	return release.LastSeenTag, nil
}

func CheckIfRepoExists(ctx context.Context, repo, token string) (*http.Response, error) {
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
