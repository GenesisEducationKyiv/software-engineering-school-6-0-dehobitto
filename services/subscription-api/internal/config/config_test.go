package config

import (
	"os"
	"reflect"
	"testing"
)

func unsetKeys(t *testing.T, keys ...string) {
	t.Helper()
	for _, key := range keys {
		key := key
		original, existed := os.LookupEnv(key)
		_ = os.Unsetenv(key)
		t.Cleanup(func() {
			if existed {
				_ = os.Setenv(key, original)
				return
			}
			_ = os.Unsetenv(key)
		})
	}
}

func TestLoad_Defaults(t *testing.T) {
	unsetKeys(t,
		"DB_HOST", "DB_PORT", "DB_USER", "DB_PASSWORD", "DB_NAME",
		"PORT", "API_KEY", "BASE_URL", "KAFKA_BROKERS", "GITHUB_TOKEN",
		"GITHUB_BASE_URL", "LOG_LEVEL", "LOG_FILE", "LOG_SIDECAR_ENABLED", "LOG_SIDECAR_URL",
	)

	cfg := Load()

	if cfg.DBName != "subber_api" {
		t.Fatalf("DBName = %q, want subber_api", cfg.DBName)
	}
	if cfg.ServerPort != "8080" {
		t.Fatalf("ServerPort = %q, want 8080", cfg.ServerPort)
	}
	if !reflect.DeepEqual(cfg.KafkaBrokers, []string{"kafka:9092"}) {
		t.Fatalf("KafkaBrokers = %#v", cfg.KafkaBrokers)
	}
	if cfg.GitHubBaseURL != "https://api.github.com" {
		t.Fatalf("GitHubBaseURL = %q", cfg.GitHubBaseURL)
	}
	if cfg.LogFile != "" {
		t.Fatalf("LogFile = %q, want empty", cfg.LogFile)
	}
	if !cfg.LogSidecarEnabled {
		t.Fatal("LogSidecarEnabled default must be true")
	}
}

func TestLoad_FromEnv(t *testing.T) {
	t.Setenv("DB_HOST", "postgres-api")
	t.Setenv("DB_NAME", "custom_api")
	t.Setenv("PORT", "9090")
	t.Setenv("API_KEY", "secret")
	t.Setenv("KAFKA_BROKERS", "kafka-1:9092,kafka-2:9092")
	t.Setenv("LOG_FILE", "/tmp/subscription-api.log")
	t.Setenv("LOG_SIDECAR_ENABLED", "false")

	cfg := Load()

	if cfg.DBHost != "postgres-api" || cfg.DBName != "custom_api" || cfg.ServerPort != "9090" {
		t.Fatalf("unexpected db/server config: %#v", cfg)
	}
	if cfg.APIKey != "secret" {
		t.Fatalf("APIKey = %q, want secret", cfg.APIKey)
	}
	if !reflect.DeepEqual(cfg.KafkaBrokers, []string{"kafka-1:9092", "kafka-2:9092"}) {
		t.Fatalf("KafkaBrokers = %#v", cfg.KafkaBrokers)
	}
	if cfg.LogFile != "/tmp/subscription-api.log" {
		t.Fatalf("LogFile = %q, want /tmp/subscription-api.log", cfg.LogFile)
	}
	if cfg.LogSidecarEnabled {
		t.Fatal("LogSidecarEnabled = true, want false")
	}
}
