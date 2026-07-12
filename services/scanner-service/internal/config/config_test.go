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
		"METRICS_PORT", "KAFKA_BROKERS", "REDIS_ADDR", "GITHUB_TOKEN", "GITHUB_BASE_URL",
		"SCANNER_BATCH_SIZE", "SCANNER_INTERVAL", "LOG_LEVEL", "LOG_FILE",
		"LOG_SIDECAR_ENABLED", "LOG_SIDECAR_URL",
	)

	cfg := Load()

	if cfg.DBName != "" {
		t.Fatalf("DBName = %q, want empty", cfg.DBName)
	}
	if cfg.RedisAddr != "" {
		t.Fatalf("RedisAddr = %q, want empty", cfg.RedisAddr)
	}
	if cfg.MetricsPort != "" {
		t.Fatalf("MetricsPort = %q, want empty", cfg.MetricsPort)
	}
	if cfg.ScannerBatchSize != 0 {
		t.Fatalf("ScannerBatchSize = %d, want 0", cfg.ScannerBatchSize)
	}
	if cfg.ScannerInterval != 0 {
		t.Fatalf("ScannerInterval = %s, want 0", cfg.ScannerInterval)
	}
	if !reflect.DeepEqual(cfg.KafkaBrokers, []string{}) {
		t.Fatalf("KafkaBrokers = %#v", cfg.KafkaBrokers)
	}
	if cfg.LogFile != "" {
		t.Fatalf("LogFile = %q, want empty", cfg.LogFile)
	}
	if cfg.LogSidecarEnabled {
		t.Fatal("LogSidecarEnabled = true, want false")
	}
}

func TestLoad_FromEnv(t *testing.T) {
	t.Setenv("DB_NAME", "custom_scanner")
	t.Setenv("REDIS_ADDR", "redis.local:6379")
	t.Setenv("METRICS_PORT", "9091")
	t.Setenv("SCANNER_BATCH_SIZE", "25")
	t.Setenv("SCANNER_INTERVAL", "45s")
	t.Setenv("LOG_FILE", "/tmp/scanner.log")
	t.Setenv("LOG_SIDECAR_ENABLED", "false")

	cfg := Load()

	if cfg.DBName != "custom_scanner" {
		t.Fatalf("DBName = %q, want custom_scanner", cfg.DBName)
	}
	if cfg.RedisAddr != "redis.local:6379" {
		t.Fatalf("RedisAddr = %q, want redis.local:6379", cfg.RedisAddr)
	}
	if cfg.ScannerBatchSize != 25 || cfg.ScannerInterval != 45*time.Second || cfg.MetricsPort != "9091" {
		t.Fatalf("scanner config = (%d, %s, %s)", cfg.ScannerBatchSize, cfg.ScannerInterval, cfg.MetricsPort)
	}
	if cfg.LogFile != "/tmp/scanner.log" {
		t.Fatalf("LogFile = %q, want /tmp/scanner.log", cfg.LogFile)
	}
	if cfg.LogSidecarEnabled {
		t.Fatal("LogSidecarEnabled = true, want false")
	}
}
