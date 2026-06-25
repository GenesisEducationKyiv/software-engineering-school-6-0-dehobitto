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
	ServerPort                string
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
		Database:                  sharedconfig.LoadDatabase(),
		Metrics:                   sharedconfig.LoadMetrics(),
		Kafka:                     sharedconfig.LoadKafka(),
		ServerPort:                env.String("PORT", ""),
		SMTPHost:                  env.String("SMTP_HOST", ""),
		SMTPPort:                  env.String("SMTP_PORT", ""),
		SMTPEmail:                 env.String("SMTP_EMAIL", ""),
		SMTPPassword:              env.String("SMTP_PASSWORD", ""),
		NotificationRetryAttempts: env.Int("NOTIFICATION_RETRY_ATTEMPTS", 0),
		NotificationRetryDelays:   env.DurationList("NOTIFICATION_RETRY_DELAYS", nil),
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
