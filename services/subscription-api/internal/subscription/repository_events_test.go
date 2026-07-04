package subscription

import (
	"encoding/json"
	"testing"
	"time"

	"subber/pkg/contracts"
)

func TestBuildRepoWatchEvent(t *testing.T) {
	occurredAt := time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)
	event, raw, err := buildRepoWatchEvent(contracts.EventRepoWatchStart, "owner/repo", "event-id", "corr-1", occurredAt)
	if err != nil {
		t.Fatalf("buildRepoWatchEvent() error = %v", err)
	}
	if event.EventType != contracts.EventRepoWatchStart || event.Source != "subscription-api" {
		t.Fatalf("metadata = %#v", event)
	}
	if event.CorrelationID != "corr-1" || event.Payload.Repo != "owner/repo" {
		t.Fatalf("payload/correlation = %#v", event)
	}

	var decoded contracts.Envelope[contracts.RepoWatchPayload]
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("raw event JSON = %v", err)
	}
	if decoded.Payload.Repo != "owner/repo" {
		t.Fatalf("decoded repo = %q", decoded.Payload.Repo)
	}
}
