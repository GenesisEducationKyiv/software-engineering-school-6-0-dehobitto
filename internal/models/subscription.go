package models

type Subscription struct {
	Email       string `json:"email"`
	Repo        string `json:"repo"` // GitHub repository in owner/repo format
	Confirmed   bool   `json:"confirmed"`
	LastSeenTag string `json:"last_seen_tag"` // Last seen release tag for "Repo"
	Token       string `json:"-"`
}

type NotificationJob struct {
	Email   string
	Message string
}
