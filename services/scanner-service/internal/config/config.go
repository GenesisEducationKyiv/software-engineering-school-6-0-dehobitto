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
	RedisAddr string
	sharedconfig.GitHub
	ScannerBatchSize int
	ScannerInterval  time.Duration
	sharedconfig.Logging
}

func Load() *Config {
	return &Config{
		Database:         sharedconfig.LoadDatabase("subber_scanner"),
		Metrics:          sharedconfig.LoadMetrics("8081"),
		Kafka:            sharedconfig.LoadKafka(),
		RedisAddr:        env.String("REDIS_ADDR", "redis:6379"),
		GitHub:           sharedconfig.LoadGitHub(),
		ScannerBatchSize: env.Int("SCANNER_BATCH_SIZE", 100),
		ScannerInterval:  env.Duration("SCANNER_INTERVAL", 30*time.Second),
		Logging:          sharedconfig.LoadLogging(),
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
