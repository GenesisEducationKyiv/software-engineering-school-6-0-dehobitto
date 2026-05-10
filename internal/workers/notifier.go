// Package workers provides background workers for notifications and scanning.
package workers

import (
	"context"
	"log"

	"subber/internal/metrics"
)

type NotificationJob struct {
	Email   string
	Message string
}

type NotifierWorker struct {
	sender EmailSender
}

func NewNotifierWorker(sender EmailSender) *NotifierWorker {
	return &NotifierWorker{sender: sender}
}

func (n *NotifierWorker) Start(ctx context.Context, jobs <-chan NotificationJob) error {
	log.Println("Notifier worker started")

	for {
		select {
		case <-ctx.Done():
			return nil
		case job, ok := <-jobs:
			if !ok {
				return nil
			}
			if err := n.sender.Send(job.Email, job.Message); err != nil {
				log.Printf("Failed to send email to %s: %v", job.Email, err)
				metrics.EmailsFailedTotal.Inc()
				continue
			}
			log.Printf("Email sent to %s", job.Email)
			metrics.EmailsSentTotal.Inc()
		}
	}
}
