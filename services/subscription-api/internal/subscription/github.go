package subscription

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"
)

var (
	ErrGitHubNotFound = errors.New("repository not found")
	ErrGitHubAPILimit = errors.New("github rate limit exceeded")
)

type GitHubClient interface {
	GetLatestTag(ctx context.Context, repo string) (string, error)
	CheckIfRepoExists(ctx context.Context, repo string) error
}

type HTTPGitHubClient struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

func NewHTTPGitHubClient(baseURL, token string) *HTTPGitHubClient {
	if baseURL == "" {
		baseURL = "https://api.github.com"
	}
	return &HTTPGitHubClient{
		baseURL:    baseURL,
		token:      token,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *HTTPGitHubClient) GetLatestTag(ctx context.Context, repo string) (string, error) {
	req, err := c.newRequest(ctx, http.MethodGet, fmt.Sprintf("%s/repos/%s/releases/latest", c.baseURL, repo))
	if err != nil {
		return "", err
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
		return "", ErrGitHubAPILimit
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

func (c *HTTPGitHubClient) CheckIfRepoExists(ctx context.Context, repo string) error {
	req, err := c.newRequest(ctx, http.MethodHead, fmt.Sprintf("%s/repos/%s", c.baseURL, repo))
	if err != nil {
		return err
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
		return ErrGitHubNotFound
	case http.StatusTooManyRequests:
		return ErrGitHubAPILimit
	default:
		return fmt.Errorf("github error: %d", resp.StatusCode)
	}
}

func (c *HTTPGitHubClient) newRequest(ctx context.Context, method, url string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Go-Subber-App")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	return req, nil
}
