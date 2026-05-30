// Package workers provides background workers for notifications and scanning.
package workers

import (
	"context"

	"subber/internal/logger"
	"subber/internal/metrics"
	"subber/internal/models"
)

var notifyLog = logger.New().WithField("component", "notifier")

type NotifierWorker struct {
	sender EmailSender
}

func NewNotifierWorker(sender EmailSender) *NotifierWorker {
	return &NotifierWorker{sender: sender}
}

func (n *NotifierWorker) Start(ctx context.Context, jobs <-chan models.NotificationJob) error {
	notifyLog.Info("notifier worker started")

	for {
		select {
		case <-ctx.Done():
			return nil
		case job, ok := <-jobs:
			if !ok {
				return nil
			}
			if err := n.sender.Send(job.Email, job.Message); err != nil {
				notifyLog.WithField("email", job.Email).WithField("repo", job.Repo).WithError(err).Error("failed to send email")
				metrics.EmailsFailedTotal.Inc()
				continue
			}
			notifyLog.WithField("email", job.Email).WithField("repo", job.Repo).Info("email sent")
			metrics.EmailsSentTotal.Inc()
		}
	}
}
