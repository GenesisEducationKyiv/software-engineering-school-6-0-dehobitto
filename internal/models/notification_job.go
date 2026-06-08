package models

// NotificationJob carries the data needed to send a release notification email.
type NotificationJob struct {
	Email       string
	Repo        string
	Message     string
	RequestID   string
	ScanCycleID string
}
