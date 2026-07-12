package scanner

import (
	"encoding/json"
	"testing"
	"time"

	"subber/pkg/contracts"
)

func TestBuildReleaseDetectedEvent(t *testing.T) {
	occurredAt := time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)
	event, raw, err := buildReleaseDetectedEvent("owner/repo", "v2.0.0", "event-id", "corr-1", occurredAt)
	if err != nil {
		t.Fatalf("buildReleaseDetectedEvent() error = %v", err)
	}
	if event.EventType != contracts.EventReleaseDetected || event.Source != "scanner-service" {
		t.Fatalf("metadata = %#v", event)
	}
	if event.Payload.Repo != "owner/repo" || event.Payload.Tag != "v2.0.0" || event.CorrelationID != "corr-1" {
		t.Fatalf("payload = %#v", event)
	}

	var decoded contracts.Envelope[contracts.ReleaseDetectedPayload]
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("raw event JSON = %v", err)
	}
	if decoded.Payload.Tag != "v2.0.0" {
		t.Fatalf("decoded tag = %q", decoded.Payload.Tag)
	}
}
