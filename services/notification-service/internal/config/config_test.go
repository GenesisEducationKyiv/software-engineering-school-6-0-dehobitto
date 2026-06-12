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

func TestLoad_Defaults(t *testing.T) {
	unsetKeys(t,
		"DB_HOST", "DB_PORT", "DB_USER", "DB_PASSWORD", "DB_NAME",
		"METRICS_PORT", "KAFKA_BROKERS", "SMTP_HOST", "SMTP_PORT", "SMTP_EMAIL", "SMTP_PASSWORD",
		"NOTIFICATION_RETRY_ATTEMPTS", "NOTIFICATION_RETRY_DELAYS",
		"LOG_LEVEL", "LOG_FILE", "LOG_SIDECAR_ENABLED", "LOG_SIDECAR_URL",
	)

	cfg := Load()

	if cfg.DBName != "subber_notifier" {
		t.Fatalf("DBName = %q, want subber_notifier", cfg.DBName)
	}
	if cfg.SMTPHost != "smtp.gmail.com" || cfg.SMTPPort != "587" {
		t.Fatalf("smtp defaults = %s:%s", cfg.SMTPHost, cfg.SMTPPort)
	}
	if cfg.MetricsPort != "8082" {
		t.Fatalf("MetricsPort = %q, want 8082", cfg.MetricsPort)
	}
	if cfg.NotificationRetryAttempts != 3 {
		t.Fatalf("NotificationRetryAttempts = %d, want 3", cfg.NotificationRetryAttempts)
	}
	wantDelays := []time.Duration{time.Minute, 10 * time.Minute, 30 * time.Minute}
	if !reflect.DeepEqual(cfg.NotificationRetryDelays, wantDelays) {
		t.Fatalf("NotificationRetryDelays = %#v", cfg.NotificationRetryDelays)
	}
	if cfg.LogFile != "" {
		t.Fatalf("LogFile = %q, want empty", cfg.LogFile)
	}
	if !cfg.LogSidecarEnabled {
		t.Fatal("LogSidecarEnabled default must be true")
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
	if cfg.LogFile != "/tmp/notifier.log" {
		t.Fatalf("LogFile = %q, want /tmp/notifier.log", cfg.LogFile)
	}
	if cfg.LogSidecarEnabled {
		t.Fatal("LogSidecarEnabled = true, want false")
	}
}
