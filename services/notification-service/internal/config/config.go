package config

import (
	"time"

	sharedconfig "subber/pkg/config"
	"subber/pkg/env"
	"subber/pkg/postgres"
)

type Config struct {
	sharedconfig.Database
	sharedconfig.Metrics
	sharedconfig.Kafka
	SMTPHost                  string
	SMTPPort                  string
	SMTPEmail                 string
	SMTPPassword              string
	NotificationRetryAttempts int
	NotificationRetryDelays   []time.Duration
	sharedconfig.Logging
}

func Load() *Config {
	return &Config{
		Database:                  sharedconfig.LoadDatabase("subber_notifier"),
		Metrics:                   sharedconfig.LoadMetrics("8082"),
		Kafka:                     sharedconfig.LoadKafka(),
		SMTPHost:                  env.String("SMTP_HOST", "smtp.gmail.com"),
		SMTPPort:                  env.String("SMTP_PORT", "587"),
		SMTPEmail:                 env.String("SMTP_EMAIL", ""),
		SMTPPassword:              env.String("SMTP_PASSWORD", ""),
		NotificationRetryAttempts: env.Int("NOTIFICATION_RETRY_ATTEMPTS", 3),
		NotificationRetryDelays:   env.DurationList("NOTIFICATION_RETRY_DELAYS", []time.Duration{time.Minute, 10 * time.Minute, 30 * time.Minute}),
		Logging:                   sharedconfig.LoadLogging(),
	}
}

func (c *Config) Postgres() postgres.Config {
	return postgres.Config{
		Host:     c.DBHost,
		Port:     c.DBPort,
		User:     c.DBUser,
		Password: c.DBPassword,
		Name:     c.DBName,
	}
}
