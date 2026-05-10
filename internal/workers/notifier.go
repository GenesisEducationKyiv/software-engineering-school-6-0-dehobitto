// Package workers provides background workers for notifications and scanning.
package workers

import (
	"context"
	"fmt"
	"log"
	"net/smtp"

	"subber/internal/config"
	"subber/internal/metrics"
)

type NotificationJob struct {
	Email   string
	Message string
}

type NotifierWorker struct {
	cfg *config.Config
}

func NewNotifierWorker(cfg *config.Config) *NotifierWorker {
	return &NotifierWorker{cfg: cfg}
}

func (n *NotifierWorker) sendEmail(to, body string) error {
	from := n.cfg.SMTPEmail
	auth := smtp.PlainAuth("", from, n.cfg.SMTPPassword, n.cfg.SMTPHost)

	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: Subber Notification\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=\"utf-8\"\r\n\r\n%s", from, to, body)

	addr := fmt.Sprintf("%s:%s", n.cfg.SMTPHost, n.cfg.SMTPPort)
	return smtp.SendMail(addr, auth, from, []string{to}, []byte(msg))
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
			if err := n.sendEmail(job.Email, job.Message); err != nil {
				log.Printf("Failed to send email to %s: %v", job.Email, err)
				metrics.EmailsFailedTotal.Inc()
				continue
			}
			log.Printf("Email sent to %s", job.Email)
			metrics.EmailsSentTotal.Inc()
		}
	}
}
