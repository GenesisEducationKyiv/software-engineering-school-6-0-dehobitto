package outbox

import (
	"context"
	"time"

	"subber/pkg/logger"
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
	log       logger.Logger
	batchSize int
	interval  time.Duration
}

func NewRelay(repo Store, publisher Publisher, batchSize int, interval time.Duration) *Relay {
	return NewRelayWithLogger(repo, publisher, nil, batchSize, interval)
}

func NewRelayWithLogger(repo Store, publisher Publisher, log logger.Logger, batchSize int, interval time.Duration) *Relay {
	if batchSize <= 0 {
		batchSize = 100
	}
	if interval <= 0 {
		interval = time.Second
	}
	if log == nil {
		log = logger.NewNoop()
	}
	return &Relay{
		repo:      repo,
		publisher: publisher,
		log:       log,
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
		r.log.WithError(err).Error("fetch outbox events failed")
		return err
	}
	for _, event := range events {
		if err := r.publisher.Publish(ctx, event.Topic, event.KafkaKey, event.Payload); err != nil {
			r.log.
				WithField("event_id", event.EventID).
				WithField("topic", event.Topic).
				WithError(err).
				Warn("publish outbox event failed")
			if markErr := r.repo.MarkFailed(ctx, event.EventID, err); markErr != nil {
				r.log.
					WithField("event_id", event.EventID).
					WithError(markErr).
					Error("mark outbox event failed")
				return markErr
			}
			break
		}
		if err := r.repo.MarkPublished(ctx, event.EventID); err != nil {
			r.log.
				WithField("event_id", event.EventID).
				WithError(err).
				Error("mark outbox event published failed")
			return err
		}
	}
	return nil
}
