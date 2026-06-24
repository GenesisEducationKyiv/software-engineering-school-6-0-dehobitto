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
		Database:         sharedconfig.LoadDatabase(),
		Metrics:          sharedconfig.LoadMetrics(),
		Kafka:            sharedconfig.LoadKafka(),
		RedisAddr:        env.String("REDIS_ADDR", ""),
		GitHub:           sharedconfig.LoadGitHub(),
		ScannerBatchSize: env.Int("SCANNER_BATCH_SIZE", 0),
		ScannerInterval:  env.Duration("SCANNER_INTERVAL", 0),
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
