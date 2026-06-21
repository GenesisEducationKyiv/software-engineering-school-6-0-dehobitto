package subscription

import (
	"encoding/json"
	"testing"
	"time"

	"subber/pkg/contracts"
)

func TestBuildRepoWatchSagaRequest(t *testing.T) {
	occurredAt := time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)
	event, raw, err := buildRepoWatchSagaRequest(contracts.RepoWatchActionStart, "owner/repo", "00000000-0000-0000-0000-000000000001", "00000000-0000-0000-0000-000000000002", occurredAt)
	if err != nil {
		t.Fatalf("buildRepoWatchSagaRequest() error = %v", err)
	}
	if event.EventType != contracts.EventRepoWatchSagaRequested || event.Source != "subscription-api" {
		t.Fatalf("metadata = %#v", event)
	}
	if event.CorrelationID != "00000000-0000-0000-0000-000000000002" ||
		event.Payload.SagaID != "00000000-0000-0000-0000-000000000001" ||
		event.Payload.Action != contracts.RepoWatchActionStart ||
		event.Payload.Repo != "owner/repo" {
		t.Fatalf("payload/correlation = %#v", event)
	}

	var decoded contracts.Envelope[contracts.RepoWatchSagaPayload]
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("raw event JSON = %v", err)
	}
	if decoded.Payload.Repo != "owner/repo" || decoded.Payload.Action != contracts.RepoWatchActionStart {
		t.Fatalf("decoded payload = %#v", decoded.Payload)
	}
}
