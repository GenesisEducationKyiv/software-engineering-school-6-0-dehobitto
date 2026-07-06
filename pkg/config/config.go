package config

import (
	"subber/pkg/env"
	"subber/pkg/postgres"
)

type Database struct {
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string
}

func LoadDatabase() Database {
	return Database{
		DBHost:     env.String("DB_HOST", ""),
		DBPort:     env.String("DB_PORT", ""),
		DBUser:     env.String("DB_USER", ""),
		DBPassword: env.String("DB_PASSWORD", ""),
		DBName:     env.String("DB_NAME", ""),
	}
}

func (c Database) Postgres() postgres.Config {
	return postgres.Config{
		Host:     c.DBHost,
		Port:     c.DBPort,
		User:     c.DBUser,
		Password: c.DBPassword,
		Name:     c.DBName,
	}
}

type Kafka struct {
	KafkaBrokers []string
}

func LoadKafka() Kafka {
	return Kafka{
		KafkaBrokers: env.CSV("KAFKA_BROKERS", ""),
	}
}

type GitHub struct {
	GitHubToken   string
	GitHubBaseURL string
}

func LoadGitHub() GitHub {
	return GitHub{
		GitHubToken:   env.String("GITHUB_TOKEN", ""),
		GitHubBaseURL: env.String("GITHUB_BASE_URL", ""),
	}
}

type Logging struct {
	LogLevel          string
	LogFile           string
	LogSidecarEnabled bool
	LogSidecarURL     string
}

func LoadLogging() Logging {
	return Logging{
		LogLevel:          env.String("LOG_LEVEL", ""),
		LogFile:           env.String("LOG_FILE", ""),
		LogSidecarEnabled: env.Bool("LOG_SIDECAR_ENABLED", false),
		LogSidecarURL:     env.String("LOG_SIDECAR_URL", ""),
	}
}

type Metrics struct {
	MetricsPort string
}

func LoadMetrics() Metrics {
	return Metrics{
		MetricsPort: env.String("METRICS_PORT", ""),
	}
}
