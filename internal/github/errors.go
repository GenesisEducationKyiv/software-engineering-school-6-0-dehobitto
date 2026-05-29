package github

import "errors"

var (
	ErrNotFound  = errors.New("repository not found")
	ErrRateLimit = errors.New("rate limit exceeded")
)
