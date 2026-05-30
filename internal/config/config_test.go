package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

// unsetKeys unsets each key for the duration of the test and restores originals via t.Cleanup.
func unsetKeys(t *testing.T, keys ...string) {
	t.Helper()
	for _, k := range keys {
		k := k
		orig, existed := os.LookupEnv(k)
		os.Unsetenv(k)
		if existed {
			t.Cleanup(func() { os.Setenv(k, orig) })
		} else {
			t.Cleanup(func() { os.Unsetenv(k) })
		}
	}
}

func TestLoadConfig_Defaults(t *testing.T) {
	unsetKeys(t,
		"DB_HOST", "DB_PORT", "DB_USER", "DB_PASSWORD", "DB_NAME",
		"PORT", "GITHUB_TOKEN", "SMTP_HOST", "SMTP_PORT",
		"SMTP_EMAIL", "SMTP_PASSWORD", "REDIS_ADDR", "API_KEY", "BASE_URL",
	)

	cfg := LoadConfig()

	cases := []struct {
		name, got, want string
	}{
		{"DBHost", cfg.DBHost, "localhost"},
		{"DBPort", cfg.DBPort, "5432"},
		{"DBUser", cfg.DBUser, "postgres"},
		{"DBPassword", cfg.DBPassword, "postgres"},
		{"DBName", cfg.DBName, "db"},
		{"ServerPort", cfg.ServerPort, "8080"},
		{"GitHubToken", cfg.GitHubToken, ""},
		{"SMTPHost", cfg.SMTPHost, "smtp.gmail.com"},
		{"SMTPPort", cfg.SMTPPort, "587"},
		{"SMTPEmail", cfg.SMTPEmail, ""},
		{"SMTPPassword", cfg.SMTPPassword, ""},
		{"RedisAddr", cfg.RedisAddr, "redis:6379"},
		{"APIKey", cfg.APIKey, ""},
		{"BaseURL", cfg.BaseURL, "http://localhost:8080"},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s = %q, want %q", c.name, c.got, c.want)
		}
	}
}

func TestLoadConfig_FromEnv(t *testing.T) {
	t.Setenv("DB_HOST", "customhost")
	t.Setenv("DB_PORT", "9999")
	t.Setenv("API_KEY", "my-secret")

	cfg := LoadConfig()

	if cfg.DBHost != "customhost" {
		t.Errorf("DBHost = %q, want customhost", cfg.DBHost)
	}
	if cfg.DBPort != "9999" {
		t.Errorf("DBPort = %q, want 9999", cfg.DBPort)
	}
	if cfg.APIKey != "my-secret" {
		t.Errorf("APIKey = %q, want my-secret", cfg.APIKey)
	}
}

func TestLoadConfig_LogLevel(t *testing.T) {
	t.Setenv("LOG_LEVEL", "debug")
	cfg := LoadConfig()
	assert.Equal(t, "debug", cfg.LogLevel)
}

func TestLoadConfig_LogFile_DefaultEmpty(t *testing.T) {
	t.Setenv("LOG_FILE", "")
	cfg := LoadConfig()
	assert.Equal(t, "", cfg.LogFile)
}

func TestLoadConfig_RabbitMQURL(t *testing.T) {
	t.Setenv("RABBITMQ_URL", "amqp://user:pass@localhost:5672/")
	cfg := LoadConfig()
	assert.Equal(t, "amqp://user:pass@localhost:5672/", cfg.RabbitMQURL)
}

func TestLoadConfig_RabbitMQURL_DefaultEmpty(t *testing.T) {
	t.Setenv("RABBITMQ_URL", "")
	cfg := LoadConfig()
	assert.Equal(t, "", cfg.RabbitMQURL)
}
