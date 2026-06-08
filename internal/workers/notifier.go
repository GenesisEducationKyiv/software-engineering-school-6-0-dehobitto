// Package workers provides background workers for notifications and scanning.
package workers

import (
	"context"

	"subber/internal/logger"
	"subber/internal/metrics"
	"subber/internal/models"
)

type NotifierWorker struct {
	sender  EmailSender
	log     logger.Logger
	metrics *metrics.Metrics
}

func NewNotifierWorker(sender EmailSender, log logger.Logger, appMetrics *metrics.Metrics) *NotifierWorker {
	if log == nil {
		log = logger.NewNoop()
	}
	if appMetrics == nil {
		appMetrics = metrics.NewNoop()
	}
	return &NotifierWorker{sender: sender, log: log, metrics: appMetrics}
}

func (n *NotifierWorker) Start(ctx context.Context, jobs <-chan models.NotificationJob) error {
	n.log.Info("notifier worker started")

	for {
		select {
		case <-ctx.Done():
			return nil
		case job, ok := <-jobs:
			if !ok {
				return nil
			}
			entry := logger.WithEmailHash(n.log, job.Email).WithField("repo", job.Repo)
			if job.RequestID != "" {
				entry = entry.WithField("request_id", job.RequestID)
			}
			if job.ScanCycleID != "" {
				entry = entry.WithField("scan_cycle_id", job.ScanCycleID)
			}
			if err := n.sender.Send(job.Email, job.Message); err != nil {
				entry.WithError(err).Error("failed to send email")
				n.metrics.EmailsFailedTotal.Inc()
				continue
			}
			entry.Info("email sent")
			n.metrics.EmailsSentTotal.Inc()
		}
	}
}
