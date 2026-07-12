//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"subber/pkg/outbox"
	outboxmigrations "subber/pkg/outbox/migrations"
)

func resetOutbox(t *testing.T) {
	t.Helper()
	if err := outboxmigrations.Run(context.Background(), sharedPool); err != nil {
		t.Fatalf("migrate outbox: %v", err)
	}
	if _, err := sharedPool.Exec(context.Background(), "TRUNCATE TABLE outbox_events"); err != nil {
		t.Fatalf("truncate outbox_events: %v", err)
	}
}

func TestOutbox_InsertTxCommitsAndFetchesInOrder(t *testing.T) {
	resetOutbox(t)
	ctx := context.Background()
	first := outboxEvent("00000000-0000-0000-0000-000000000001", "topic-a", "key-a", `{"n":1}`, time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC))
	second := outboxEvent("00000000-0000-0000-0000-000000000002", "topic-b", "key-b", `{"n":2}`, first.OccurredAt.Add(time.Second))

	insertOutboxEvent(t, first)
	insertOutboxEvent(t, second)

	events, err := outbox.NewRepository(sharedPool).FetchUnpublished(ctx, 10)
	if err != nil {
		t.Fatalf("FetchUnpublished() error = %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("events = %d, want 2", len(events))
	}
	if events[0].EventID != first.EventID || events[1].EventID != second.EventID {
		t.Fatalf("order = [%s, %s]", events[0].EventID, events[1].EventID)
	}
	if events[0].Topic != "topic-a" || events[0].KafkaKey != "key-a" || jsonNumber(t, events[0].Payload) != 1 {
		t.Fatalf("first event = %#v", events[0])
	}
	if events[0].LockedUntil == nil || !events[0].LockedUntil.After(time.Now().UTC()) {
		t.Fatalf("event was not claimed with future lock: %#v", events[0].LockedUntil)
	}
}

func TestOutbox_InsertTxRollsBackAndRejectsInvalidJSON(t *testing.T) {
	resetOutbox(t)
	ctx := context.Background()

	tx, err := sharedPool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	event := outboxEvent("00000000-0000-0000-0000-000000000003", "topic", "key", `{"ok":true}`, time.Now().UTC())
	if err := outbox.InsertTx(ctx, tx, event); err != nil {
		t.Fatalf("InsertTx() error = %v", err)
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatalf("rollback tx: %v", err)
	}

	events, err := outbox.NewRepository(sharedPool).FetchUnpublished(ctx, 10)
	if err != nil {
		t.Fatalf("FetchUnpublished() error = %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("events after rollback = %d, want 0", len(events))
	}

	tx, err = sharedPool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	bad := outboxEvent("00000000-0000-0000-0000-000000000004", "topic", "key", `not-json`, time.Now().UTC())
	if err := outbox.InsertTx(ctx, tx, bad); err == nil {
		t.Fatal("expected invalid JSON error, got nil")
	}
}

func TestOutbox_InsertTxIsIdempotentByEventID(t *testing.T) {
	resetOutbox(t)
	event := outboxEvent("00000000-0000-0000-0000-000000000005", "topic", "key", `{"n":1}`, time.Now().UTC())

	insertOutboxEvent(t, event)
	insertOutboxEvent(t, event)

	var count int
	if err := sharedPool.QueryRow(context.Background(), "SELECT COUNT(*) FROM outbox_events WHERE event_id = $1", event.EventID).Scan(&count); err != nil {
		t.Fatalf("count outbox events: %v", err)
	}
	if count != 1 {
		t.Fatalf("outbox rows = %d, want 1", count)
	}
}

func TestOutbox_FetchClaimsRowsUntilFailureOrPublish(t *testing.T) {
	resetOutbox(t)
	ctx := context.Background()
	repo := outbox.NewRepository(sharedPool)
	first := outboxEvent("00000000-0000-0000-0000-000000000006", "topic", "key", `{"n":1}`, time.Now().UTC())
	second := outboxEvent("00000000-0000-0000-0000-000000000007", "topic", "key", `{"n":2}`, time.Now().UTC().Add(time.Second))
	insertOutboxEvent(t, first)
	insertOutboxEvent(t, second)

	events, err := repo.FetchUnpublished(ctx, 1)
	if err != nil {
		t.Fatalf("first FetchUnpublished() error = %v", err)
	}
	if len(events) != 1 || events[0].EventID != first.EventID {
		t.Fatalf("first claim = %#v", events)
	}

	events, err = repo.FetchUnpublished(ctx, 10)
	if err != nil {
		t.Fatalf("second FetchUnpublished() error = %v", err)
	}
	if len(events) != 1 || events[0].EventID != second.EventID {
		t.Fatalf("second claim = %#v, want only second event", events)
	}

	publishErr := errors.New("kafka down")
	if err := repo.MarkFailed(ctx, first.EventID, publishErr); err != nil {
		t.Fatalf("MarkFailed() error = %v", err)
	}
	events, err = repo.FetchUnpublished(ctx, 10)
	if err != nil {
		t.Fatalf("third FetchUnpublished() error = %v", err)
	}
	if len(events) != 1 || events[0].EventID != first.EventID || events[0].PublishAttempts != 1 {
		t.Fatalf("third claim after failure = %#v", events)
	}
	if events[0].LastError == nil || *events[0].LastError != publishErr.Error() {
		t.Fatalf("last error = %#v, want %q", events[0].LastError, publishErr.Error())
	}
}

func TestOutbox_MarkPublishedExcludesEventAndClearsError(t *testing.T) {
	resetOutbox(t)
	ctx := context.Background()
	repo := outbox.NewRepository(sharedPool)
	event := outboxEvent("00000000-0000-0000-0000-000000000008", "topic", "key", `{"n":1}`, time.Now().UTC())
	insertOutboxEvent(t, event)

	if _, err := repo.FetchUnpublished(ctx, 1); err != nil {
		t.Fatalf("FetchUnpublished() error = %v", err)
	}
	if err := repo.MarkFailed(ctx, event.EventID, errors.New("temporary failure")); err != nil {
		t.Fatalf("MarkFailed() error = %v", err)
	}
	if _, err := repo.FetchUnpublished(ctx, 1); err != nil {
		t.Fatalf("FetchUnpublished() after failure error = %v", err)
	}
	if err := repo.MarkPublished(ctx, event.EventID); err != nil {
		t.Fatalf("MarkPublished() error = %v", err)
	}

	events, err := repo.FetchUnpublished(ctx, 10)
	if err != nil {
		t.Fatalf("FetchUnpublished() after publish error = %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("events after publish = %#v, want none", events)
	}

	var lastError *string
	var lockedUntil *time.Time
	if err := sharedPool.QueryRow(ctx, "SELECT last_error, locked_until FROM outbox_events WHERE event_id = $1", event.EventID).Scan(&lastError, &lockedUntil); err != nil {
		t.Fatalf("query published event: %v", err)
	}
	if lastError != nil || lockedUntil != nil {
		t.Fatalf("published event should clear last_error and locked_until, got last_error=%v locked_until=%v", lastError, lockedUntil)
	}
}

func insertOutboxEvent(t *testing.T, event outbox.Event) {
	t.Helper()
	ctx := context.Background()
	tx, err := sharedPool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	if err := outbox.InsertTx(ctx, tx, event); err != nil {
		t.Fatalf("InsertTx() error = %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit tx: %v", err)
	}
}

func outboxEvent(eventID, topic, key, payload string, occurredAt time.Time) outbox.Event {
	return outbox.Event{
		EventID:       eventID,
		EventType:     "TestEvent",
		OccurredAt:    occurredAt,
		Source:        "integration-test",
		CorrelationID: uuid.NewString(),
		Topic:         topic,
		KafkaKey:      key,
		Payload:       []byte(payload),
	}
}

func jsonNumber(t *testing.T, raw []byte) float64 {
	t.Helper()
	var decoded struct {
		N float64 `json:"n"`
	}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("decode JSON payload: %v", err)
	}
	return decoded.N
}
