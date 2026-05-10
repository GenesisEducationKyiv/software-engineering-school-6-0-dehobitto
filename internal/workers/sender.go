package workers

import (
	"fmt"
	"net/smtp"

	"subber/internal/config"
)

// EmailSender is the interface for sending notification emails.
// Implement it to add new delivery channels without touching NotifierWorker.
type EmailSender interface {
	Send(to, body string) error
}

type SMTPSender struct {
	cfg *config.Config
}

func NewSMTPSender(cfg *config.Config) *SMTPSender {
	return &SMTPSender{cfg: cfg}
}

func (s *SMTPSender) Send(to, body string) error {
	from := s.cfg.SMTPEmail
	auth := smtp.PlainAuth("", from, s.cfg.SMTPPassword, s.cfg.SMTPHost)

	msg := fmt.Sprintf(
		"From: %s\r\nTo: %s\r\nSubject: Subber Notification\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=\"utf-8\"\r\n\r\n%s",
		from, to, body,
	)

	addr := fmt.Sprintf("%s:%s", s.cfg.SMTPHost, s.cfg.SMTPPort)
	return smtp.SendMail(addr, auth, from, []string{to}, []byte(msg))
}
