package delivery

import (
	"context"
	"time"

	"subber/pkg/contracts"
	"subber/pkg/logger"
)

type EmailSender interface {
	Send(to, body string) error
}

type Store interface {
	UpsertPending(ctx context.Context, payload contracts.NotificationSendRequestedPayload, correlationID string) (Delivery, error)
	MarkSent(ctx context.Context, idempotencyKey string) error
	MarkFailed(ctx context.Context, idempotencyKey string, cause error, nextAttemptAt time.Time) error
	MarkDead(ctx context.Context, payload contracts.NotificationSendRequestedPayload, correlationID string, cause error) error
}

type Service struct {
	repo        Store
	sender      EmailSender
	log         logger.Logger
	maxAttempts int
	retryDelays []time.Duration
}

func NewService(repo Store, sender EmailSender, log logger.Logger, maxAttempts int, retryDelays []time.Duration) *Service {
	if log == nil {
		log = logger.NewNoop()
	}
	if maxAttempts <= 0 {
		maxAttempts = 3
	}
	return &Service{
		repo:        repo,
		sender:      sender,
		log:         log,
		maxAttempts: maxAttempts,
		retryDelays: retryDelays,
	}
}

func (s *Service) Process(ctx context.Context, payload contracts.NotificationSendRequestedPayload, correlationID string) error {
	delivery, err := s.repo.UpsertPending(ctx, payload, correlationID)
	if err != nil {
		return err
	}
	if delivery.Status == StatusSent {
		s.log.WithField("notification_id", delivery.NotificationID).Info("duplicate notification already sent")
		return nil
	}
	if delivery.Status == StatusDead {
		s.log.WithField("notification_id", delivery.NotificationID).Info("duplicate notification already dead-lettered")
		return nil
	}

	if err := s.sender.Send(payload.RecipientEmail, payload.Message); err != nil {
		return s.handleFailure(ctx, payload, delivery.CorrelationID, delivery.AttemptCount, err)
	}

	if err := s.repo.MarkSent(ctx, payload.IdempotencyKey); err != nil {
		return err
	}
	NotificationSentTotal.Inc()
	s.log.WithField("notification_id", payload.NotificationID).WithField("repo", payload.Repo).Info("notification sent")
	return nil
}

func (s *Service) handleFailure(ctx context.Context, payload contracts.NotificationSendRequestedPayload, correlationID string, currentAttempts int, cause error) error {
	nextAttempt := currentAttempts + 1
	if nextAttempt >= s.maxAttempts {
		if err := s.repo.MarkDead(ctx, payload, correlationID, cause); err != nil {
			return err
		}
		NotificationDeadTotal.Inc()
		return nil
	}

	nextAttemptAt := time.Now().UTC().Add(s.retryDelay(nextAttempt))
	if err := s.repo.MarkFailed(ctx, payload.IdempotencyKey, cause, nextAttemptAt); err != nil {
		return err
	}
	return nil
}

func (s *Service) retryDelay(attempt int) time.Duration {
	if attempt <= 0 || len(s.retryDelays) == 0 {
		return time.Minute
	}
	index := attempt - 1
	if index >= len(s.retryDelays) {
		index = len(s.retryDelays) - 1
	}
	return s.retryDelays[index]
}
