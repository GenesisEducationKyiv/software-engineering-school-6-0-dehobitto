package delivery

import (
	"context"
	"time"

	"subber/pkg/logger"
)

type DueRetryStore interface {
	EnqueueDueRetries(ctx context.Context, limit int) (int, error)
}

type RetryScheduler struct {
	store     DueRetryStore
	log       logger.Logger
	batchSize int
	interval  time.Duration
}

func NewRetryScheduler(store DueRetryStore, log logger.Logger, batchSize int, interval time.Duration) *RetryScheduler {
	if batchSize <= 0 {
		batchSize = 100
	}
	if interval <= 0 {
		interval = time.Second
	}
	if log == nil {
		log = logger.NewNoop()
	}
	return &RetryScheduler{
		store:     store,
		log:       log,
		batchSize: batchSize,
		interval:  interval,
	}
}

func (s *RetryScheduler) Start(ctx context.Context) error {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			enqueued, err := s.store.EnqueueDueRetries(ctx, s.batchSize)
			if err != nil {
				s.log.WithError(err).Error("enqueue due notification retries failed")
				return err
			}
			if enqueued > 0 {
				s.log.WithField("count", enqueued).Info("enqueued due notification retries")
			}
		}
	}
}
