package models

type GitHubRelease struct {
	Repo        string `json:"repo"`
	LastSeenTag string `json:"tag_name"`
}
