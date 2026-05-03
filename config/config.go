// Package config provides configuration loading and management for the application.
package config

import (
	"os"
)

// Config contains all app settings.
type Config struct {
	DBHost       string
	DBPort       string
	DBUser       string
	DBPassword   string
	DBName       string
	ServerPort   string
	GitHubToken  string
	SMTPHost     string
	SMTPPort     string
	SMTPEmail    string
	SMTPPassword string
	RedisAddr    string
	APIKey       string
	SchemasPath  string
	BaseURL      string
}

// LoadConfig reads .env and returns filled Config structure.
func LoadConfig() *Config {
	return &Config{
		DBHost:       getEnv("DB_HOST", "localhost"),
		DBPort:       getEnv("DB_PORT", "5432"),
		DBUser:       getEnv("DB_USER", "postgres"),
		DBPassword:   getEnv("DB_PASSWORD", "postgres"),
		DBName:       getEnv("DB_NAME", "db"),
		ServerPort:   getEnv("PORT", "8080"),
		GitHubToken:  getEnv("GITHUB_TOKEN", ""),
		SMTPHost:     getEnv("SMTP_HOST", "smtp.gmail.com"),
		SMTPPort:     getEnv("SMTP_PORT", "587"),
		SMTPEmail:    getEnv("SMTP_EMAIL", ""),
		SMTPPassword: getEnv("SMTP_PASSWORD", ""),
		RedisAddr:    getEnv("REDIS_ADDR", "redis:6379"),
		APIKey:       getEnv("API_KEY", ""),
		SchemasPath:  getEnv("SCHEMAS_PATH", "infra/database/schemas.sql"),
		BaseURL:      getEnv("BASE_URL", "http://localhost:8080"),
	}
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}
