package config

import (
	"os"
	"testing"
)

func TestLoadConfig_Defaults(t *testing.T) {
	os.Clearenv()

	cfg := LoadConfig()

	if cfg.DBHost != "localhost" {
		t.Errorf("expected default DBHost=localhost, got %s", cfg.DBHost)
	}
	if cfg.DBPort != "5432" {
		t.Errorf("expected default DBPort=5432, got %s", cfg.DBPort)
	}
	if cfg.ServerPort != "8080" {
		t.Errorf("expected default ServerPort=8080, got %s", cfg.ServerPort)
	}
	if cfg.RedisAddr != "redis:6379" {
		t.Errorf("expected default RedisAddr=redis:6379, got %s", cfg.RedisAddr)
	}
	if cfg.APIKey != "" {
		t.Errorf("expected default APIKey to be empty, got %s", cfg.APIKey)
	}
}

func TestLoadConfig_FromEnv(t *testing.T) {
	_ = os.Setenv("DB_HOST", "customhost")
	_ = os.Setenv("DB_PORT", "9999")
	_ = os.Setenv("API_KEY", "my-secret")
	defer func() {
		_ = os.Unsetenv("DB_HOST")
		_ = os.Unsetenv("DB_PORT")
		_ = os.Unsetenv("API_KEY")
	}()

	cfg := LoadConfig()

	if cfg.DBHost != "customhost" {
		t.Errorf("expected DBHost=customhost, got %s", cfg.DBHost)
	}
	if cfg.DBPort != "9999" {
		t.Errorf("expected DBPort=9999, got %s", cfg.DBPort)
	}
	if cfg.APIKey != "my-secret" {
		t.Errorf("expected APIKey=my-secret, got %s", cfg.APIKey)
	}
}
