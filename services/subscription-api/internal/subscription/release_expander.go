package subscription

import (
	"context"
	"fmt"

	"subber/pkg/contracts"
)

type SubscriberStore interface {
	GetSubscribers(ctx context.Context, repo string) ([]string, error)
}

type ReleaseNotificationPublisher interface {
	PublishReleaseNotification(ctx context.Context, email, repo, tag, correlationID string) error
}

type ReleaseExpander struct {
	store     SubscriberStore
	publisher ReleaseNotificationPublisher
}

func NewReleaseExpander(store SubscriberStore, publisher ReleaseNotificationPublisher) *ReleaseExpander {
	return &ReleaseExpander{store: store, publisher: publisher}
}

func (e *ReleaseExpander) Expand(ctx context.Context, event contracts.Envelope[contracts.ReleaseDetectedPayload]) error {
	subscribers, err := e.store.GetSubscribers(ctx, event.Payload.Repo)
	if err != nil {
		return fmt.Errorf("get release subscribers: %w", err)
	}
	for _, email := range subscribers {
		if err := e.publisher.PublishReleaseNotification(ctx, email, event.Payload.Repo, event.Payload.Tag, event.CorrelationID); err != nil {
			return fmt.Errorf("publish release notification: %w", err)
		}
	}
	return nil
}
