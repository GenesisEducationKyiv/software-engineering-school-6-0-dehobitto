// Package models defines the data structures used throughout the application.
package models

// GitHubRelease represents a GitHub repository and its latest known release tag.
type GitHubRelease struct {
	Repo        string `json:"repo"`
	LastSeenTag string `json:"tag_name"`
}
