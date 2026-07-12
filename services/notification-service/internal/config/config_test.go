package config

import (
	"os"
	"reflect"
	"testing"
	"time"
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
		"METRICS_PORT", "KAFKA_BROKERS", "SMTP_HOST", "SMTP_PORT", "SMTP_EMAIL", "SMTP_PASSWORD",
		"NOTIFICATION_RETRY_ATTEMPTS", "NOTIFICATION_RETRY_DELAYS", "NOTIFICATION_TRANSPORT", "GRPC_PORT",
		"LOG_LEVEL", "LOG_FILE", "LOG_SIDECAR_ENABLED", "LOG_SIDECAR_URL",
	)

	cfg := Load()

	if cfg.DBName != "" {
		t.Fatalf("DBName = %q, want empty", cfg.DBName)
	}
	if cfg.SMTPHost != "" || cfg.SMTPPort != "" {
		t.Fatalf("smtp config without env = %s:%s, want empty", cfg.SMTPHost, cfg.SMTPPort)
	}
	if cfg.MetricsPort != "" {
		t.Fatalf("MetricsPort = %q, want empty", cfg.MetricsPort)
	}
	if cfg.NotificationRetryAttempts != 0 {
		t.Fatalf("NotificationRetryAttempts = %d, want 0", cfg.NotificationRetryAttempts)
	}
	if cfg.NotificationRetryDelays != nil {
		t.Fatalf("NotificationRetryDelays = %#v, want nil", cfg.NotificationRetryDelays)
	}
	if cfg.NotificationTransport != "kafka" {
		t.Fatalf("NotificationTransport = %q, want kafka", cfg.NotificationTransport)
	}
	if cfg.GRPCPort != "9093" {
		t.Fatalf("GRPCPort = %q, want 9093", cfg.GRPCPort)
	}
	if cfg.LogFile != "" {
		t.Fatalf("LogFile = %q, want empty", cfg.LogFile)
	}
	if cfg.LogSidecarEnabled {
		t.Fatal("LogSidecarEnabled = true, want false")
	}
}

func TestLoad_FromEnv(t *testing.T) {
	t.Setenv("DB_NAME", "custom_notifier")
	t.Setenv("SMTP_HOST", "smtp.example.com")
	t.Setenv("METRICS_PORT", "9092")
	t.Setenv("SMTP_PORT", "2525")
	t.Setenv("SMTP_EMAIL", "sender@example.com")
	t.Setenv("SMTP_PASSWORD", "secret")
	t.Setenv("NOTIFICATION_RETRY_ATTEMPTS", "5")
	t.Setenv("NOTIFICATION_RETRY_DELAYS", "2m,15m")
	t.Setenv("NOTIFICATION_TRANSPORT", "grpc")
	t.Setenv("GRPC_PORT", "9191")
	t.Setenv("LOG_FILE", "/tmp/notifier.log")
	t.Setenv("LOG_SIDECAR_ENABLED", "false")

	cfg := Load()

	if cfg.DBName != "custom_notifier" {
		t.Fatalf("DBName = %q, want custom_notifier", cfg.DBName)
	}
	if cfg.SMTPHost != "smtp.example.com" || cfg.SMTPPort != "2525" || cfg.MetricsPort != "9092" {
		t.Fatalf("smtp config = %s:%s metrics=%s", cfg.SMTPHost, cfg.SMTPPort, cfg.MetricsPort)
	}
	if cfg.SMTPEmail != "sender@example.com" || cfg.SMTPPassword != "secret" {
		t.Fatal("smtp credentials were not loaded from env")
	}
	if cfg.NotificationRetryAttempts != 5 {
		t.Fatalf("NotificationRetryAttempts = %d, want 5", cfg.NotificationRetryAttempts)
	}
	if !reflect.DeepEqual(cfg.NotificationRetryDelays, []time.Duration{2 * time.Minute, 15 * time.Minute}) {
		t.Fatalf("NotificationRetryDelays = %#v", cfg.NotificationRetryDelays)
	}
	if cfg.NotificationTransport != "grpc" {
		t.Fatalf("NotificationTransport = %q, want grpc", cfg.NotificationTransport)
	}
	if cfg.GRPCPort != "9191" {
		t.Fatalf("GRPCPort = %q, want 9191", cfg.GRPCPort)
	}
	if cfg.LogFile != "/tmp/notifier.log" {
		t.Fatalf("LogFile = %q, want /tmp/notifier.log", cfg.LogFile)
	}
	if cfg.LogSidecarEnabled {
		t.Fatal("LogSidecarEnabled = true, want false")
	}
}
