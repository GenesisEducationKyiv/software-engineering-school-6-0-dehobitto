package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"subber/pkg/logger"
	"subber/services/scanner-service/internal/cache"
)

const (
	defaultBaseURL = "https://api.github.com"
	userAgent      = "Go-Subber-App"
	cacheKeyPrefix = "github:latest_tag:"
)

type ReleaseProvider interface {
	GetLatestTag(ctx context.Context, repo string) (string, error)
}

type HTTPReleaseProvider struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

func NewHTTPReleaseProvider(baseURL, token string) *HTTPReleaseProvider {
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	return &HTTPReleaseProvider{
		baseURL:    baseURL,
		token:      token,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

func (p *HTTPReleaseProvider) GetLatestTag(ctx context.Context, repo string) (string, error) {
	link := fmt.Sprintf("%s/repos/%s/releases/latest", p.baseURL, repo)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, link, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", userAgent)
	if p.token != "" {
		req.Header.Set("Authorization", "Bearer "+p.token)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close() //nolint:errcheck

	switch resp.StatusCode {
	case http.StatusNotFound:
		return "", nil
	case http.StatusTooManyRequests:
		return "", fmt.Errorf("github rate limit exceeded")
	case http.StatusOK:
	default:
		return "", fmt.Errorf("github error: %d", resp.StatusCode)
	}

	var release struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", err
	}
	return release.TagName, nil
}

type CachedReleaseProvider struct {
	cache cache.Cache
	next  ReleaseProvider
	ttl   time.Duration
	log   logger.Logger
}

func NewCachedReleaseProvider(c cache.Cache, next ReleaseProvider, ttl time.Duration, log logger.Logger) *CachedReleaseProvider {
	if ttl <= 0 {
		ttl = 45 * time.Second
	}
	if log == nil {
		log = logger.NewNoop()
	}
	return &CachedReleaseProvider{cache: c, next: next, ttl: ttl, log: log}
}

func (p *CachedReleaseProvider) GetLatestTag(ctx context.Context, repo string) (string, error) {
	key := cacheKeyPrefix + repo
	if p.cache != nil {
		cached, err := p.cache.Get(ctx, key)
		if err == nil && cached != "" {
			return cached, nil
		}
		if err != nil {
			p.log.WithField("repo", repo).WithError(err).Warn("cache get failed")
		}
	}

	tag, err := p.next.GetLatestTag(ctx, repo)
	if err != nil {
		return "", err
	}
	if p.cache != nil && tag != "" {
		if err := p.cache.Set(ctx, key, tag, p.ttl); err != nil {
			p.log.WithField("repo", repo).WithError(err).Warn("cache set failed")
		}
	}
	return tag, nil
}
