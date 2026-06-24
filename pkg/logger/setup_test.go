package logger

import (
	"os"
	"strings"
	"testing"
)

func TestConfigureRejectsInvalidLevel(t *testing.T) {
	cleanup, err := Configure("not-a-level", false, "")
	defer cleanup()
	if err == nil {
		t.Fatal("expected invalid log level error, got nil")
	}
}

func TestConfigureWritesJSONLogFile(t *testing.T) {
	file, err := os.CreateTemp(t.TempDir(), "subber-*.log")
	if err != nil {
		t.Fatalf("create temp log file: %v", err)
	}
	path := file.Name()
	if err := file.Close(); err != nil {
		t.Fatalf("close temp log file: %v", err)
	}

	cleanup, err := Configure("info", false, "", path)
	if err != nil {
		t.Fatalf("Configure() error = %v", err)
	}
	New().WithField("service", "test").Info("hello")
	cleanup()

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read log file: %v", err)
	}
	logLine := string(raw)
	if !strings.Contains(logLine, `"level":"info"`) || !strings.Contains(logLine, `"msg":"hello"`) || !strings.Contains(logLine, `"service":"test"`) {
		t.Fatalf("log line = %s", logLine)
	}
}
