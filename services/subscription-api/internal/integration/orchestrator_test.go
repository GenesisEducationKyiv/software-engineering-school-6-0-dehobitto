//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"

	"subber/pkg/contracts"
	"subber/services/subscription-api/internal/watchsaga"
)

func TestOrchestrator_HandleRequestCreatesSagaAndCommand(t *testing.T) {
	o := newTestOrchestrator(t)
	sagaID := uuid.MustParse("00000000-0000-0000-0000-000000000101").String()
	correlationID := uuid.MustParse("00000000-0000-0000-0000-000000000202").String()
	event := contracts.Envelope[contracts.RepoWatchSagaPayload]{
		EventType:     contracts.EventRepoWatchSagaRequested,
		CorrelationID: correlationID,
		Payload: contracts.RepoWatchSagaPayload{
			SagaID: sagaID,
			Action: contracts.RepoWatchActionStart,
			Repo:   "owner/repo",
		},
	}

	if err := o.HandleRequest(context.Background(), event); err != nil {
		t.Fatalf("HandleRequest() error = %v", err)
	}

	instance := fetchSagaInstance(t, sagaID)
	if instance.SagaID != sagaID || instance.Repo != "owner/repo" || instance.Action != contracts.RepoWatchActionStart {
		t.Fatalf("saga instance = %#v", instance)
	}
	if instance.Status != "command_sent" || instance.Attempts != 1 || instance.CorrelationID != correlationID {
		t.Fatalf("saga instance status/attempts/correlation = %#v", instance)
	}

	outboxEvent := fetchOutboxEvent(t, sagaID)
	if outboxEvent.EventType != contracts.EventStartWatchingRepo || outboxEvent.Topic != contracts.TopicWatchlistCommands {
		t.Fatalf("outbox event = %#v", outboxEvent)
	}
	var payload contracts.Envelope[contracts.RepoWatchCommandPayload]
	if err := json.Unmarshal(outboxEvent.Payload, &payload); err != nil {
		t.Fatalf("decode outbox payload: %v", err)
	}
	if payload.EventType != contracts.EventStartWatchingRepo || payload.Payload.Repo != "owner/repo" {
		t.Fatalf("decoded payload = %#v", payload)
	}
}

func TestOrchestrator_HandleAckCompletesAndRetries(t *testing.T) {
	o := newTestOrchestrator(t)
	sagaID := uuid.MustParse("00000000-0000-0000-0000-000000000303").String()
	startRequest := contracts.Envelope[contracts.RepoWatchSagaPayload]{
		EventType:     contracts.EventRepoWatchSagaRequested,
		CorrelationID: uuid.MustParse("00000000-0000-0000-0000-000000000304").String(),
		Payload: contracts.RepoWatchSagaPayload{
			SagaID: sagaID,
			Action: contracts.RepoWatchActionStart,
			Repo:   "owner/repo",
		},
	}
	if err := o.HandleRequest(context.Background(), startRequest); err != nil {
		t.Fatalf("HandleRequest() error = %v", err)
	}

	ack := contracts.Envelope[contracts.RepoWatchAckPayload]{
		EventType: contracts.EventRepoWatchStarted,
		Payload: contracts.RepoWatchAckPayload{
			SagaID: sagaID,
			Action: contracts.RepoWatchActionStart,
			Repo:   "owner/repo",
		},
	}
	if err := o.HandleAck(context.Background(), ack); err != nil {
		t.Fatalf("HandleAck(started) error = %v", err)
	}
	instance := fetchSagaInstance(t, sagaID)
	if instance.Status != "completed" || instance.LastError != nil {
		t.Fatalf("completed saga instance = %#v", instance)
	}

	sagaIDRetry := uuid.MustParse("00000000-0000-0000-0000-000000000305").String()
	request := contracts.Envelope[contracts.RepoWatchSagaPayload]{
		EventType:     contracts.EventRepoWatchSagaRequested,
		CorrelationID: uuid.MustParse("00000000-0000-0000-0000-000000000306").String(),
		Payload: contracts.RepoWatchSagaPayload{
			SagaID: sagaIDRetry,
			Action: contracts.RepoWatchActionStop,
			Repo:   "owner/repo",
		},
	}
	if err := o.HandleRequest(context.Background(), request); err != nil {
		t.Fatalf("HandleRequest() retry saga error = %v", err)
	}
	failedAck := contracts.Envelope[contracts.RepoWatchAckPayload]{
		EventType: contracts.EventRepoWatchFailed,
		Payload: contracts.RepoWatchAckPayload{
			SagaID: sagaIDRetry,
			Action: contracts.RepoWatchActionStop,
			Repo:   "owner/repo",
			Error:  "scanner down",
		},
	}
	if err := o.HandleAck(context.Background(), failedAck); err != nil {
		t.Fatalf("HandleAck(failed) error = %v", err)
	}
	instance = fetchSagaInstance(t, sagaIDRetry)
	if instance.Status != "retrying" || instance.LastError == nil || *instance.LastError != "scanner down" {
		t.Fatalf("retrying saga instance = %#v", instance)
	}
}

func TestOrchestrator_HandleAckRejectsUnknownSaga(t *testing.T) {
	o := newTestOrchestrator(t)

	ack := contracts.Envelope[contracts.RepoWatchAckPayload]{
		EventType: contracts.EventRepoWatchStarted,
		Payload: contracts.RepoWatchAckPayload{
			SagaID: uuid.MustParse("00000000-0000-0000-0000-000000000999").String(),
			Action: contracts.RepoWatchActionStart,
			Repo:   "owner/repo",
		},
	}

	if err := o.HandleAck(context.Background(), ack); err == nil {
		t.Fatal("HandleAck() error = nil, want error")
	}
}

func TestOrchestrator_HandleAckRejectsPayloadMismatch(t *testing.T) {
	o := newTestOrchestrator(t)
	sagaID := uuid.MustParse("00000000-0000-0000-0000-000000001001").String()
	request := contracts.Envelope[contracts.RepoWatchSagaPayload]{
		EventType:     contracts.EventRepoWatchSagaRequested,
		CorrelationID: uuid.MustParse("00000000-0000-0000-0000-000000001002").String(),
		Payload: contracts.RepoWatchSagaPayload{
			SagaID: sagaID,
			Action: contracts.RepoWatchActionStart,
			Repo:   "owner/repo",
		},
	}
	if err := o.HandleRequest(context.Background(), request); err != nil {
		t.Fatalf("HandleRequest() error = %v", err)
	}

	ack := contracts.Envelope[contracts.RepoWatchAckPayload]{
		EventType: contracts.EventRepoWatchStarted,
		Payload: contracts.RepoWatchAckPayload{
			SagaID: sagaID,
			Action: contracts.RepoWatchActionStart,
			Repo:   "owner/other-repo",
		},
	}
	if err := o.HandleAck(context.Background(), ack); err == nil {
		t.Fatal("HandleAck() error = nil, want error")
	}

	instance := fetchSagaInstance(t, sagaID)
	if instance.Status != "command_sent" || instance.Repo != "owner/repo" || instance.Action != contracts.RepoWatchActionStart {
		t.Fatalf("saga instance after rejected ack = %#v", instance)
	}
}

func TestOrchestrator_RetryDueResendsAndExpires(t *testing.T) {
	o := newTestOrchestrator(t)
	now := time.Now().UTC()

	dueSagaID := uuid.MustParse("00000000-0000-0000-0000-000000000401").String()
	expiredSagaID := uuid.MustParse("00000000-0000-0000-0000-000000000402").String()

	insertSagaInstance(t, dueSagaID, "owner/repo", contracts.RepoWatchActionStart, "command_sent", 1, now.Add(-time.Minute), now.Add(time.Hour), "00000000-0000-0000-0000-000000000403", nil)
	insertSagaInstance(t, expiredSagaID, "owner/repo", contracts.RepoWatchActionStop, "retrying", 2, now.Add(-time.Minute), now.Add(-time.Minute), "00000000-0000-0000-0000-000000000404", strPtr("scanner down"))

	if err := o.RetryDue(context.Background(), 10); err != nil {
		t.Fatalf("RetryDue() error = %v", err)
	}

	dueInstance := fetchSagaInstance(t, dueSagaID)
	if dueInstance.Status != "retrying" || dueInstance.Attempts != 2 || dueInstance.LastError != nil {
		t.Fatalf("due instance after retry = %#v", dueInstance)
	}
	expiredInstance := fetchSagaInstance(t, expiredSagaID)
	if expiredInstance.Status != "failed" || expiredInstance.LastError == nil || *expiredInstance.LastError != "timeout waiting for scanner ack" {
		t.Fatalf("expired instance after retry = %#v", expiredInstance)
	}

	if got := countOutboxBySagaID(t, dueSagaID); got != 1 {
		t.Fatalf("outbox rows for due saga = %d, want 1", got)
	}
	if got := countOutboxBySagaID(t, expiredSagaID); got != 0 {
		t.Fatalf("outbox rows for expired saga = %d, want 0", got)
	}
}

func newTestOrchestrator(t *testing.T) *watchsaga.Orchestrator {
	t.Helper()
	if sharedPool == nil {
		t.Fatal("sharedPool is nil")
	}
	truncateWatchSagaTables(t)
	return watchsaga.New(sharedPool)
}

func truncateWatchSagaTables(t *testing.T) {
	t.Helper()
	if _, err := sharedPool.Exec(context.Background(), "TRUNCATE TABLE saga_instances, outbox_events"); err != nil {
		t.Fatalf("truncate tables: %v", err)
	}
}

func fetchSagaInstance(t *testing.T, sagaID string) watchsaga.Instance {
	t.Helper()
	var instance watchsaga.Instance
	err := sharedPool.QueryRow(context.Background(), `
SELECT saga_id::text, repo, action, status, attempts, correlation_id::text, deadline_at, last_error
FROM saga_instances
WHERE saga_id = $1
`, sagaID).Scan(
		&instance.SagaID,
		&instance.Repo,
		&instance.Action,
		&instance.Status,
		&instance.Attempts,
		&instance.CorrelationID,
		&instance.DeadlineAt,
		&instance.LastError,
	)
	if err != nil {
		t.Fatalf("fetch saga instance: %v", err)
	}
	return instance
}

type outboxRecord struct {
	EventType string
	Topic     string
	Payload   []byte
}

func fetchOutboxEvent(t *testing.T, sagaID string) outboxRecord {
	t.Helper()
	var payload []byte
	var eventType string
	var topic string
	err := sharedPool.QueryRow(context.Background(), `
SELECT event_type, topic, payload
FROM outbox_events
WHERE payload->'payload'->>'saga_id' = $1
ORDER BY occurred_at DESC, event_id DESC
LIMIT 1
`, sagaID).Scan(&eventType, &topic, &payload)
	if err != nil {
		t.Fatalf("fetch outbox event: %v", err)
	}
	return outboxRecord{EventType: eventType, Topic: topic, Payload: payload}
}

func insertSagaInstance(t *testing.T, sagaID, repo, action, status string, attempts int, nextRetryAt, deadlineAt time.Time, correlationID string, lastError *string) {
	t.Helper()
	_, err := sharedPool.Exec(context.Background(), `
INSERT INTO saga_instances (
	saga_id,
	saga_type,
	repo,
	action,
	status,
	attempts,
	correlation_id,
	next_retry_at,
	deadline_at,
	last_error
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
`, sagaID, watchsaga.TypeRepoWatch, repo, action, status, attempts, correlationID, nextRetryAt, deadlineAt, lastError)
	if err != nil {
		t.Fatalf("insert saga instance: %v", err)
	}
}

func countOutboxBySagaID(t *testing.T, sagaID string) int {
	t.Helper()
	var count int
	err := sharedPool.QueryRow(context.Background(), `
SELECT COUNT(*)
FROM outbox_events
WHERE payload->'payload'->>'saga_id' = $1
`, sagaID).Scan(&count)
	if err != nil {
		t.Fatalf("count outbox by saga id: %v", err)
	}
	return count
}

func strPtr(s string) *string {
	return &s
}
