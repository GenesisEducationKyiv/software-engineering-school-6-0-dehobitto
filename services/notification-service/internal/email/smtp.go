package email

import (
	"fmt"
	"net/smtp"
)

type Config struct {
	SMTPHost     string
	SMTPPort     string
	SMTPEmail    string
	SMTPPassword string
}

type Sender interface {
	Send(to, body string) error
}

type SMTPSender struct {
	cfg Config
}

func NewSMTPSender(cfg Config) *SMTPSender {
	return &SMTPSender{cfg: cfg}
}

func (s *SMTPSender) Send(to, body string) error {
	from := senderAddress(s.cfg)
	var auth smtp.Auth
	if s.cfg.SMTPEmail != "" && s.cfg.SMTPPassword != "" {
		auth = smtp.PlainAuth("", from, s.cfg.SMTPPassword, s.cfg.SMTPHost)
	}

	msg := fmt.Sprintf(
		"From: %s\r\nTo: %s\r\nSubject: Subber Notification\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=\"utf-8\"\r\n\r\n%s",
		from, to, body,
	)

	addr := fmt.Sprintf("%s:%s", s.cfg.SMTPHost, s.cfg.SMTPPort)
	return smtp.SendMail(addr, auth, from, []string{to}, []byte(msg))
}

func senderAddress(cfg Config) string {
	if cfg.SMTPEmail != "" {
		return cfg.SMTPEmail
	}
	return "subber@localhost"
}
