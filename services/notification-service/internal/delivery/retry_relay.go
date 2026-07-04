package delivery

import (
	"context"
	"time"

	"subber/pkg/contracts"
)

type RetryStore interface {
	FetchDueRetries(ctx context.Context, limit int) ([]ScheduledRetry, error)
}

type RetryProcessor interface {
	Process(ctx context.Context, payload contracts.NotificationSendRequestedPayload, correlationID string) error
}

type RetryRelay struct {
	repo      RetryStore
	processor RetryProcessor
	batchSize int
	interval  time.Duration
}

func NewRetryRelay(repo RetryStore, processor RetryProcessor, batchSize int, interval time.Duration) *RetryRelay {
	if batchSize <= 0 {
		batchSize = 100
	}
	if interval <= 0 {
		interval = time.Second
	}
	return &RetryRelay{
		repo:      repo,
		processor: processor,
		batchSize: batchSize,
		interval:  interval,
	}
}

func (r *RetryRelay) Start(ctx context.Context) error {
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := r.ProcessOnce(ctx); err != nil {
				return err
			}
		}
	}
}

func (r *RetryRelay) ProcessOnce(ctx context.Context) error {
	payloads, err := r.repo.FetchDueRetries(ctx, r.batchSize)
	if err != nil {
		return err
	}
	for _, retry := range payloads {
		if err := r.processor.Process(ctx, retry.Payload, retry.CorrelationID); err != nil {
			return err
		}
	}
	return nil
}
