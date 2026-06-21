package contracts

import "time"

const (
	TopicWatchlistSagaRequests = "subber.watchlist.saga.requests"
	TopicWatchlistCommands     = "subber.watchlist.saga.commands"
	TopicWatchlistSagaEvents   = "subber.watchlist.saga.events"
	TopicWatchlistCommandsDLQ  = "subber.watchlist.commands.dlq"
	TopicReleaseEvents         = "subber.release.events"
	TopicNotificationCommands  = "subber.notification.commands"
	TopicNotificationRetry1m   = "subber.notification.retry.1m"
	TopicNotificationRetry10m  = "subber.notification.retry.10m"
	TopicNotificationDLQ       = "subber.notification.dlq"

	RepoWatchActionStart = "start_watch"
	RepoWatchActionStop  = "stop_watch"

	EventRepoWatchSagaRequested = "RepoWatchSagaRequested"
	EventStartWatchingRepo      = "StartWatchingRepo"
	EventStopWatchingRepo       = "StopWatchingRepo"
	EventRepoWatchStarted       = "RepoWatchStarted"
	EventRepoWatchStopped       = "RepoWatchStopped"
	EventRepoWatchFailed        = "RepoWatchFailed"

	EventReleaseDetected       = "ReleaseDetected"
	EventNotificationRequested = "NotificationSendRequested"
	EventConsumerDeadLettered  = "ConsumerMessageDeadLettered"
)

type Envelope[T any] struct {
	EventID       string     `json:"event_id"`
	EventType     string     `json:"event_type"`
	OccurredAt    time.Time  `json:"occurred_at"`
	Source        string     `json:"source"`
	CorrelationID string     `json:"correlation_id"`
	NotBefore     *time.Time `json:"not_before,omitempty"`
	Payload       T          `json:"payload"`
}

type RepoWatchPayload struct {
	Repo string `json:"repo"`
}

type RepoWatchSagaPayload struct {
	SagaID string `json:"saga_id"`
	Action string `json:"action"`
	Repo   string `json:"repo"`
}

type RepoWatchCommandPayload struct {
	SagaID string `json:"saga_id"`
	Action string `json:"action"`
	Repo   string `json:"repo"`
}

type RepoWatchAckPayload struct {
	SagaID string `json:"saga_id"`
	Action string `json:"action"`
	Repo   string `json:"repo"`
	Error  string `json:"error,omitempty"`
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

type DeadLetterPayload struct {
	OriginalTopic     string    `json:"original_topic"`
	OriginalPartition int       `json:"original_partition"`
	OriginalOffset    int64     `json:"original_offset"`
	OriginalKey       string    `json:"original_key"`
	OriginalValue     []byte    `json:"original_value"`
	ConsumerGroup     string    `json:"consumer_group"`
	Cause             string    `json:"cause"`
	FailedAt          time.Time `json:"failed_at"`
}
