package config

import (
	sharedconfig "subber/pkg/config"
	"subber/pkg/env"
	"subber/pkg/postgres"
)

type Config struct {
	sharedconfig.Database
	ServerPort string
	APIKey     string
	BaseURL    string
	sharedconfig.Kafka
	sharedconfig.GitHub
	sharedconfig.Logging
}

func Load() *Config {
	return &Config{
		Database:   sharedconfig.LoadDatabase("subber_api"),
		ServerPort: env.String("PORT", "8080"),
		APIKey:     env.String("API_KEY", ""),
		BaseURL:    env.String("BASE_URL", "http://localhost:8080"),
		Kafka:      sharedconfig.LoadKafka(),
		GitHub:     sharedconfig.LoadGitHub(),
		Logging:    sharedconfig.LoadLogging(),
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
