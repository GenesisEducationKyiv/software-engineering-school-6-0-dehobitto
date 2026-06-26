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

func TestLoad_EmptyWhenEnvMissing(t *testing.T) {
	unsetKeys(t,
		"DB_HOST", "DB_PORT", "DB_USER", "DB_PASSWORD", "DB_NAME",
		"PORT", "API_KEY", "BASE_URL", "KAFKA_BROKERS", "GITHUB_TOKEN",
		"GITHUB_BASE_URL", "NOTIFICATION_TRANSPORT", "NOTIFICATION_GRPC_ADDR",
		"LOG_LEVEL", "LOG_FILE", "LOG_SIDECAR_ENABLED", "LOG_SIDECAR_URL",
	)

	cfg := Load()

	if cfg.DBName != "" {
		t.Fatalf("DBName = %q, want empty", cfg.DBName)
	}
	if cfg.ServerPort != "" {
		t.Fatalf("ServerPort = %q, want empty", cfg.ServerPort)
	}
	if !reflect.DeepEqual(cfg.KafkaBrokers, []string{}) {
		t.Fatalf("KafkaBrokers = %#v", cfg.KafkaBrokers)
	}
	if cfg.GitHubBaseURL != "" {
		t.Fatalf("GitHubBaseURL = %q, want empty", cfg.GitHubBaseURL)
	}
	if cfg.NotificationTransport != "kafka" {
		t.Fatalf("NotificationTransport = %q, want kafka", cfg.NotificationTransport)
	}
	if cfg.NotificationGRPCAddress != "notification-service:9093" {
		t.Fatalf("NotificationGRPCAddress = %q, want notification-service:9093", cfg.NotificationGRPCAddress)
	}
	if cfg.LogFile != "" {
		t.Fatalf("LogFile = %q, want empty", cfg.LogFile)
	}
	if cfg.LogSidecarEnabled {
		t.Fatal("LogSidecarEnabled = true, want false")
	}
}

func TestLoad_FromEnv(t *testing.T) {
	t.Setenv("DB_HOST", "postgres-api")
	t.Setenv("DB_NAME", "custom_api")
	t.Setenv("PORT", "9090")
	t.Setenv("API_KEY", "secret")
	t.Setenv("KAFKA_BROKERS", "kafka-1:9092,kafka-2:9092")
	t.Setenv("NOTIFICATION_TRANSPORT", "grpc")
	t.Setenv("NOTIFICATION_GRPC_ADDR", "notifier:9191")
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
	if cfg.NotificationTransport != "grpc" || cfg.NotificationGRPCAddress != "notifier:9191" {
		t.Fatalf("unexpected notification config: transport=%q addr=%q", cfg.NotificationTransport, cfg.NotificationGRPCAddress)
	}
	if cfg.LogFile != "/tmp/subscription-api.log" {
		t.Fatalf("LogFile = %q, want /tmp/subscription-api.log", cfg.LogFile)
	}
	if cfg.LogSidecarEnabled {
		t.Fatal("LogSidecarEnabled = true, want false")
	}
}
