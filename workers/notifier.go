// Package workers provides background workers for notifications and scanning.
package workers

import (
	"fmt"
	"log"
	"net/smtp"

	"subber/config"
	"subber/middleware"
)

// NotificationJob represents an email notification to be sent.
type NotificationJob struct {
	Email   string
	Message string
}

// NotifierWorker consumes notification jobs from a channel and sends emails.
type NotifierWorker struct {
	cfg *config.Config
}

// NewNotifierWorker creates a new NotifierWorker with the given config.
func NewNotifierWorker(cfg *config.Config) *NotifierWorker {
	return &NotifierWorker{
		cfg: cfg,
	}
}

func (n *NotifierWorker) sendEmail(to, body string) error {
	from := n.cfg.SMTPEmail
	auth := smtp.PlainAuth("", from, n.cfg.SMTPPassword, n.cfg.SMTPHost)

	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: Subber Notification\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=\"utf-8\"\r\n\r\n%s", from, to, body)

	addr := fmt.Sprintf("%s:%s", n.cfg.SMTPHost, n.cfg.SMTPPort)
	return smtp.SendMail(addr, auth, from, []string{to}, []byte(msg))
}

// Start begins processing notification jobs from the channel.
func (n *NotifierWorker) Start(jobs <-chan NotificationJob) {
	log.Println("Notifier worker started")

	for job := range jobs {
		if err := n.sendEmail(job.Email, job.Message); err != nil {
			log.Printf("Failed to send email to %s: %v", job.Email, err)
			middleware.EmailsFailedTotal.Inc()
			continue
		}
		log.Printf("Email sent to %s", job.Email)
		middleware.EmailsSentTotal.Inc()
	}
}
