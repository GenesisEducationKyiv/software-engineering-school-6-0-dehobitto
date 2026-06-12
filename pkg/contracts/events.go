package contracts

import "time"

const (
	TopicWatchlistEvents       = "subber.watchlist.events"
	TopicReleaseEvents         = "subber.release.events"
	TopicNotificationCommands  = "subber.notification.commands"
	TopicNotificationRetry1m   = "subber.notification.retry.1m"
	TopicNotificationRetry10m  = "subber.notification.retry.10m"
	TopicNotificationDLQ       = "subber.notification.dlq"
	EventRepoWatchStart        = "RepoWatchStartRequested"
	EventRepoWatchStop         = "RepoWatchStopRequested"
	EventReleaseDetected       = "ReleaseDetected"
	EventNotificationRequested = "NotificationSendRequested"
)

type Envelope[T any] struct {
	EventID       string    `json:"event_id"`
	EventType     string    `json:"event_type"`
	OccurredAt    time.Time `json:"occurred_at"`
	Source        string    `json:"source"`
	CorrelationID string    `json:"correlation_id"`
	Payload       T         `json:"payload"`
}

type RepoWatchPayload struct {
	Repo string `json:"repo"`
}

type ReleaseDetectedPayload struct {
	Repo string `json:"repo"`
	Tag  string `json:"tag"`
}

type NotificationSendRequestedPayload struct {
	NotificationID string `json:"notification_id"`
	IdempotencyKey string `json:"idempotency_key"`
	RecipientEmail string `json:"recipient_email"`
	EmailHash      string `json:"email_hash"`
	Repo           string `json:"repo"`
	Tag            string `json:"tag"`
	Message        string `json:"message"`
}
