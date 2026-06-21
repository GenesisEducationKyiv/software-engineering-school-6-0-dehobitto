//go:build integration

package watchsaga

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"subber/pkg/contracts"
	"subber/services/subscription-api/internal/dbmigrations"
)

var sharedPool *pgxpool.Pool

func TestMain(m *testing.M) {
	os.Exit(run(m))
}

func run(m *testing.M) int {
	ctx := context.Background()

	pgContainer, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("subscription_api_watchsaga_test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		panic(err)
	}
	defer pgContainer.Terminate(ctx) //nolint:errcheck

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		panic(err)
	}

	sharedPool, err = pgxpool.New(ctx, connStr)
	if err != nil {
		panic(err)
	}
	defer sharedPool.Close()

	if err := dbmigrations.Run(ctx, sharedPool); err != nil {
		panic(err)
	}

	return m.Run()
}

func TestOrchestrator_HandleRequestCreatesSagaAndCommand(t *testing.T) {
	o := newTestOrchestrator(t)
	o.now = func() time.Time { return time.Date(2026, 6, 22, 12, 0, 0, 0, time.UTC) }
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
	if instance.Status != StatusCommandSent || instance.Attempts != 1 || instance.CorrelationID != correlationID {
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
	o.now = func() time.Time { return time.Date(2026, 6, 22, 12, 0, 0, 0, time.UTC) }
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
	if instance.Status != StatusCompleted || instance.LastError != nil {
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
	if instance.Status != StatusRetrying || instance.LastError == nil || *instance.LastError != "scanner down" {
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
	if instance.Status != StatusCommandSent || instance.Repo != "owner/repo" || instance.Action != contracts.RepoWatchActionStart {
		t.Fatalf("saga instance after rejected ack = %#v", instance)
	}
}

func TestOrchestrator_RetryDueResendsAndExpires(t *testing.T) {
	o := newTestOrchestrator(t)
	now := time.Date(2026, 6, 22, 12, 0, 0, 0, time.UTC)
	o.now = func() time.Time { return now }

	dueSagaID := uuid.MustParse("00000000-0000-0000-0000-000000000401").String()
	expiredSagaID := uuid.MustParse("00000000-0000-0000-0000-000000000402").String()

	insertSagaInstance(t, dueSagaID, "owner/repo", contracts.RepoWatchActionStart, StatusCommandSent, 1, now.Add(-time.Minute), now.Add(time.Hour), "00000000-0000-0000-0000-000000000403", nil)
	insertSagaInstance(t, expiredSagaID, "owner/repo", contracts.RepoWatchActionStop, StatusRetrying, 2, now.Add(-time.Minute), now.Add(-time.Second), "00000000-0000-0000-0000-000000000404", strPtr("scanner down"))

	if err := o.RetryDue(context.Background(), 10); err != nil {
		t.Fatalf("RetryDue() error = %v", err)
	}

	dueInstance := fetchSagaInstance(t, dueSagaID)
	if dueInstance.Status != StatusRetrying || dueInstance.Attempts != 2 || dueInstance.LastError != nil {
		t.Fatalf("due instance after retry = %#v", dueInstance)
	}
	expiredInstance := fetchSagaInstance(t, expiredSagaID)
	if expiredInstance.Status != StatusFailed || expiredInstance.LastError == nil || *expiredInstance.LastError != "timeout waiting for scanner ack" {
		t.Fatalf("expired instance after retry = %#v", expiredInstance)
	}

	if got := countOutboxBySagaID(t, dueSagaID); got != 1 {
		t.Fatalf("outbox rows for due saga = %d, want 1", got)
	}
	if got := countOutboxBySagaID(t, expiredSagaID); got != 0 {
		t.Fatalf("outbox rows for expired saga = %d, want 0", got)
	}
}

func newTestOrchestrator(t *testing.T) *Orchestrator {
	t.Helper()
	if sharedPool == nil {
		t.Fatal("sharedPool is nil")
	}
	truncateWatchSagaTables(t)
	o := New(sharedPool)
	o.deadline = 24 * time.Hour
	return o
}

func truncateWatchSagaTables(t *testing.T) {
	t.Helper()
	if _, err := sharedPool.Exec(context.Background(), "TRUNCATE TABLE saga_instances, outbox_events"); err != nil {
		t.Fatalf("truncate tables: %v", err)
	}
}

func fetchSagaInstance(t *testing.T, sagaID string) Instance {
	t.Helper()
	var instance Instance
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
`, sagaID, TypeRepoWatch, repo, action, status, attempts, correlationID, nextRetryAt, deadlineAt, lastError)
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
