package config

import (
	"reflect"
	"testing"
)

func TestLoadSharedConfigFromEnv(t *testing.T) {
	t.Setenv("DB_HOST", "postgres")
	t.Setenv("DB_PORT", "5432")
	t.Setenv("DB_USER", "user")
	t.Setenv("DB_PASSWORD", "secret")
	t.Setenv("DB_NAME", "subber")
	t.Setenv("KAFKA_BROKERS", "kafka-1:9092,kafka-2:9092")
	t.Setenv("GITHUB_TOKEN", "token")
	t.Setenv("GITHUB_BASE_URL", "https://github.example")
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("LOG_FILE", "/tmp/app.log")
	t.Setenv("LOG_SIDECAR_ENABLED", "true")
	t.Setenv("LOG_SIDECAR_URL", "http://vector:8686")
	t.Setenv("METRICS_PORT", "9090")

	db := LoadDatabase()
	if db.DBHost != "postgres" || db.DBPort != "5432" || db.DBUser != "user" || db.DBPassword != "secret" || db.DBName != "subber" {
		t.Fatalf("database config = %#v", db)
	}

	kafka := LoadKafka()
	if !reflect.DeepEqual(kafka.KafkaBrokers, []string{"kafka-1:9092", "kafka-2:9092"}) {
		t.Fatalf("kafka brokers = %#v", kafka.KafkaBrokers)
	}

	github := LoadGitHub()
	if github.GitHubToken != "token" || github.GitHubBaseURL != "https://github.example" {
		t.Fatalf("github config = %#v", github)
	}

	logging := LoadLogging()
	if logging.LogLevel != "debug" || logging.LogFile != "/tmp/app.log" || !logging.LogSidecarEnabled || logging.LogSidecarURL != "http://vector:8686" {
		t.Fatalf("logging config = %#v", logging)
	}

	metrics := LoadMetrics()
	if metrics.MetricsPort != "9090" {
		t.Fatalf("metrics port = %q, want 9090", metrics.MetricsPort)
	}
}

func TestLoadSharedConfigEmptyWithoutEnv(t *testing.T) {
	for _, key := range []string{
		"DB_HOST", "DB_PORT", "DB_USER", "DB_PASSWORD", "DB_NAME",
		"KAFKA_BROKERS", "GITHUB_TOKEN", "GITHUB_BASE_URL",
		"LOG_LEVEL", "LOG_FILE", "LOG_SIDECAR_ENABLED", "LOG_SIDECAR_URL",
		"METRICS_PORT",
	} {
		t.Setenv(key, "")
	}

	db := LoadDatabase()
	if db.DBHost != "" || db.DBPort != "" || db.DBUser != "" || db.DBPassword != "" || db.DBName != "" {
		t.Fatalf("database config = %#v, want empty", db)
	}
	if got := LoadKafka().KafkaBrokers; !reflect.DeepEqual(got, []string{}) {
		t.Fatalf("kafka brokers = %#v, want empty", got)
	}
	if github := LoadGitHub(); github.GitHubToken != "" || github.GitHubBaseURL != "" {
		t.Fatalf("github config = %#v, want empty", github)
	}
	if logging := LoadLogging(); logging.LogLevel != "" || logging.LogFile != "" || logging.LogSidecarEnabled || logging.LogSidecarURL != "" {
		t.Fatalf("logging config = %#v, want empty", logging)
	}
	if metrics := LoadMetrics(); metrics.MetricsPort != "" {
		t.Fatalf("metrics config = %#v, want empty", metrics)
	}
}
