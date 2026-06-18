package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"subber/internal/logger"
)

type recordedLog struct {
	level  string
	msg    string
	fields map[string]any
}

type recordingLogger struct {
	fields  map[string]any
	records *[]recordedLog
}

func newRecordingLogger() recordingLogger {
	records := make([]recordedLog, 0)
	return recordingLogger{fields: map[string]any{}, records: &records}
}

func (l recordingLogger) WithField(key string, value any) logger.Logger {
	next := recordingLogger{fields: map[string]any{}, records: l.records}
	for k, v := range l.fields {
		next.fields[k] = v
	}
	next.fields[key] = value
	return next
}

func (l recordingLogger) WithError(err error) logger.Logger {
	return l.WithField("error", err)
}

func (l recordingLogger) Info(msg string)  { l.record("info", msg) }
func (l recordingLogger) Warn(msg string)  { l.record("warn", msg) }
func (l recordingLogger) Error(msg string) { l.record("error", msg) }
func (l recordingLogger) Fatal(msg string) { l.record("fatal", msg) }

func (l recordingLogger) record(level, msg string) {
	fields := map[string]any{}
	for k, v := range l.fields {
		fields[k] = v
	}
	*l.records = append(*l.records, recordedLog{level: level, msg: msg, fields: fields})
}

func TestLoggingMiddlewareRecordsSafeRequestFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	log := newRecordingLogger()

	r := gin.New()
	r.Use(RequestIDMiddleware())
	r.Use(LoggingMiddleware(log))
	r.GET("/api/subscriptions/", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/subscriptions/?email=user@example.com", nil)
	req.Header.Set("User-Agent", "test-agent")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if len(*log.records) != 1 {
		t.Fatalf("records = %d, want 1", len(*log.records))
	}

	fields := (*log.records)[0].fields
	if fields["route"] != "/api/subscriptions/" {
		t.Fatalf("route = %v, want /api/subscriptions/", fields["route"])
	}
	if fields["has_query"] != true {
		t.Fatalf("has_query = %v, want true", fields["has_query"])
	}
	if fields["user_agent"] != "test-agent" {
		t.Fatalf("user_agent = %v, want test-agent", fields["user_agent"])
	}
	if _, ok := fields["request_id"]; !ok {
		t.Fatal("request_id field missing")
	}
	if _, ok := fields["client_ip_hash"]; !ok {
		t.Fatal("client_ip_hash field missing")
	}
	if _, ok := fields["ip"]; ok {
		t.Fatal("raw ip field must not be logged")
	}
}
