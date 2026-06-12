package outbox

import (
	"context"
	"time"
)

type Publisher interface {
	Publish(ctx context.Context, topic, key string, value []byte) error
}

type Store interface {
	FetchUnpublished(ctx context.Context, limit int) ([]Event, error)
	MarkPublished(ctx context.Context, eventID string) error
	MarkFailed(ctx context.Context, eventID string, cause error) error
}

type Relay struct {
	repo      Store
	publisher Publisher
	batchSize int
	interval  time.Duration
}

func NewRelay(repo Store, publisher Publisher, batchSize int, interval time.Duration) *Relay {
	if batchSize <= 0 {
		batchSize = 100
	}
	if interval <= 0 {
		interval = time.Second
	}
	return &Relay{
		repo:      repo,
		publisher: publisher,
		batchSize: batchSize,
		interval:  interval,
	}
}

func (r *Relay) Start(ctx context.Context) error {
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := r.PublishOnce(ctx); err != nil {
				return err
			}
		}
	}
}

func (r *Relay) PublishOnce(ctx context.Context) error {
	events, err := r.repo.FetchUnpublished(ctx, r.batchSize)
	if err != nil {
		return err
	}
	for _, event := range events {
		if err := r.publisher.Publish(ctx, event.Topic, event.KafkaKey, event.Payload); err != nil {
			if markErr := r.repo.MarkFailed(ctx, event.EventID, err); markErr != nil {
				return markErr
			}
			continue
		}
		if err := r.repo.MarkPublished(ctx, event.EventID); err != nil {
			return err
		}
	}
	return nil
}
