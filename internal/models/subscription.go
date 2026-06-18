package models

// Subscription represents a user's subscription to a GitHub repository's releases.
type Subscription struct {
	Email       string `json:"email"`         // Email address
	Repo        string `json:"repo"`          // GitHub repository in owner/repo format
	Confirmed   bool   `json:"confirmed"`     // Whether the subscription is confirmed
	LastSeenTag string `json:"last_seen_tag"` // Last seen release tag for "Repo"
	Token       string `json:"-"`
}
