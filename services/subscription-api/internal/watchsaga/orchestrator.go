package watchsaga

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"subber/pkg/contracts"
	"subber/pkg/outbox"
)

const (
	TypeRepoWatch     = "repo_watch"
	StatusCommandSent = "command_sent"
	StatusRetrying    = "retrying"
	StatusCompleted   = "completed"
	StatusFailed      = "failed"
	DefaultRetryLimit = 50
	DefaultDeadline   = 24 * time.Hour
)

var retryDelays = []time.Duration{
	time.Minute,
	5 * time.Minute,
	15 * time.Minute,
	time.Hour,
}

type Instance struct {
	SagaID        string
	Repo          string
	Action        string
	Status        string
	Attempts      int
	CorrelationID string
	DeadlineAt    time.Time
	LastError     *string
}

type Orchestrator struct {
	transactions transactionBeginner
	now          func() time.Time
	deadline     time.Duration
}

type transactionBeginner interface {
	BeginTx(ctx context.Context) (pgx.Tx, error)
}

type poolTransactionBeginner struct {
	pool *pgxpool.Pool
}

func (b poolTransactionBeginner) BeginTx(ctx context.Context) (pgx.Tx, error) {
	tx, err := b.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return tx, nil
}

func New(pool *pgxpool.Pool) *Orchestrator {
	return NewWithTransactions(poolTransactionBeginner{pool: pool})
}

func NewWithTransactions(transactions transactionBeginner) *Orchestrator {
	return &Orchestrator{
		transactions: transactions,
		now:          func() time.Time { return time.Now().UTC() },
		deadline:     DefaultDeadline,
	}
}

func (o *Orchestrator) HandleRequest(ctx context.Context, event contracts.Envelope[contracts.RepoWatchSagaPayload]) error {
	if event.EventType != contracts.EventRepoWatchSagaRequested {
		return fmt.Errorf("unsupported saga request event type %q", event.EventType)
	}
	if err := validateAction(event.Payload.Action); err != nil {
		return err
	}
	if _, err := uuid.Parse(event.Payload.SagaID); err != nil {
		return fmt.Errorf("invalid saga id: %w", err)
	}
	if event.Payload.Repo == "" {
		return fmt.Errorf("repo is required")
	}

	tx, err := o.transactions.BeginTx(ctx)
	if err != nil {
		return fmt.Errorf("begin saga request: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	now := o.now()
	deadlineAt := now.Add(o.deadline)
	nextRetryAt := now.Add(retryDelay(1))
	tag, err := tx.Exec(ctx, `
	INSERT INTO saga_instances (
		saga_id,
		saga_type,
		repo,
		action,
		status,
		attempts,
		correlation_id,
		next_retry_at,
		deadline_at
	) VALUES ($1, $2, $3, $4, $5, 1, $6, $7, $8)
	ON CONFLICT (saga_id) DO NOTHING
	`,
		event.Payload.SagaID, TypeRepoWatch, event.Payload.Repo, event.Payload.Action, StatusCommandSent, event.CorrelationID, nextRetryAt, deadlineAt)
	if err != nil {
		return fmt.Errorf("create saga instance: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return tx.Commit(ctx)
	}

	if err := prepareWatchCommand(ctx, tx, event.Payload.SagaID, event.Payload.Action, event.Payload.Repo, event.CorrelationID, now); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (o *Orchestrator) HandleAck(ctx context.Context, event contracts.Envelope[contracts.RepoWatchAckPayload]) error {
	if event.Payload.SagaID == "" {
		return fmt.Errorf("saga id is required")
	}
	if err := validateAction(event.Payload.Action); err != nil {
		return err
	}

	tx, err := o.transactions.BeginTx(ctx)
	if err != nil {
		return fmt.Errorf("begin saga ack: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	instance, err := loadSagaInstanceForUpdate(ctx, tx, event.Payload.SagaID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("saga instance %s not found", event.Payload.SagaID)
		}
		return fmt.Errorf("load saga instance: %w", err)
	}
	if instance.Repo != event.Payload.Repo || instance.Action != event.Payload.Action {
		return fmt.Errorf("saga instance %s payload mismatch", event.Payload.SagaID)
	}

	switch event.EventType {
	case contracts.EventRepoWatchStarted, contracts.EventRepoWatchStopped:
		return o.handleSuccessAck(ctx, tx, event, instance)
	case contracts.EventRepoWatchFailed:
		return o.handleFailedAck(ctx, tx, event, instance)
	default:
		return fmt.Errorf("unsupported saga ack event type %q", event.EventType)
	}
}

func (o *Orchestrator) handleSuccessAck(ctx context.Context, tx pgx.Tx, event contracts.Envelope[contracts.RepoWatchAckPayload], instance Instance) error {
	ackEventType, err := repoWatchAckEventType(instance.Action)
	if err != nil {
		return err
	}
	if event.EventType != ackEventType {
		return fmt.Errorf("unexpected saga ack event type %q for action %q", event.EventType, instance.Action)
	}
	if instance.Status == StatusCompleted || instance.Status == StatusFailed {
		return tx.Commit(ctx)
	}
	tag, err := tx.Exec(ctx, `
UPDATE saga_instances
SET status = $2,
	last_error = NULL,
	updated_at = now()
WHERE saga_id = $1
	AND repo = $3
	AND action = $4
	AND status NOT IN ($5, $6)
`, event.Payload.SagaID, StatusCompleted, instance.Repo, instance.Action, StatusCompleted, StatusFailed)
	if err != nil {
		return fmt.Errorf("complete saga instance: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("saga instance %s was not updated", event.Payload.SagaID)
	}
	return tx.Commit(ctx)
}

func (o *Orchestrator) handleFailedAck(ctx context.Context, tx pgx.Tx, event contracts.Envelope[contracts.RepoWatchAckPayload], instance Instance) error {
	now := o.now()
	nextRetryAt := now.Add(retryDelay(1))
	if event.Payload.Error == "" {
		event.Payload.Error = "watch command failed"
	}
	if instance.Status == StatusCompleted || instance.Status == StatusFailed {
		return tx.Commit(ctx)
	}
	tag, err := tx.Exec(ctx, `
UPDATE saga_instances
SET status = CASE WHEN deadline_at <= $2 THEN $3 ELSE $4 END,
	next_retry_at = CASE WHEN deadline_at <= $2 THEN NULL::timestamptz ELSE $5::timestamptz END,
	last_error = $6,
	updated_at = now()
WHERE saga_id = $1
	AND repo = $7
	AND action = $8
	AND status NOT IN ($3, $9)
`, event.Payload.SagaID, now, StatusFailed, StatusRetrying, nextRetryAt, event.Payload.Error, instance.Repo, instance.Action, StatusCompleted)
	if err != nil {
		return fmt.Errorf("mark saga failed ack: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("saga instance %s was not updated", event.Payload.SagaID)
	}
	return tx.Commit(ctx)
}

func (o *Orchestrator) RetryDue(ctx context.Context, limit int) error {
	if limit <= 0 {
		limit = DefaultRetryLimit
	}

	tx, err := o.transactions.BeginTx(ctx)
	if err != nil {
		return fmt.Errorf("begin saga retries: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	now := o.now()
	rows, err := tx.Query(ctx, `
SELECT saga_id::text, repo, action, status, attempts, correlation_id::text, deadline_at, last_error
FROM saga_instances
WHERE status IN ($1, $2)
	AND next_retry_at <= $3
ORDER BY next_retry_at, created_at
FOR UPDATE SKIP LOCKED
LIMIT $4
`, StatusCommandSent, StatusRetrying, now, limit)
	if err != nil {
		return fmt.Errorf("select due saga retries: %w", err)
	}
	defer rows.Close()

	var due []Instance
	for rows.Next() {
		var instance Instance
		if err := rows.Scan(
			&instance.SagaID,
			&instance.Repo,
			&instance.Action,
			&instance.Status,
			&instance.Attempts,
			&instance.CorrelationID,
			&instance.DeadlineAt,
			&instance.LastError,
		); err != nil {
			return fmt.Errorf("scan due saga retry: %w", err)
		}
		due = append(due, instance)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate due saga retries: %w", err)
	}

	for _, instance := range due {
		if !now.Before(instance.DeadlineAt) {
			if _, err := tx.Exec(ctx, `
UPDATE saga_instances
SET status = $2,
	next_retry_at = NULL,
	last_error = $3,
	updated_at = now()
WHERE saga_id = $1
`, instance.SagaID, StatusFailed, "timeout waiting for scanner ack"); err != nil {
				return fmt.Errorf("expire saga instance: %w", err)
			}
			continue
		}

		attempts := instance.Attempts + 1
		nextRetryAt := now.Add(retryDelay(attempts))
		if err := prepareWatchCommand(ctx, tx, instance.SagaID, instance.Action, instance.Repo, instance.CorrelationID, now); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
UPDATE saga_instances
SET status = $2,
	attempts = $3,
	next_retry_at = $4,
	updated_at = now()
WHERE saga_id = $1
`, instance.SagaID, StatusRetrying, attempts, nextRetryAt); err != nil {
			return fmt.Errorf("update retried saga instance: %w", err)
		}
	}

	return tx.Commit(ctx)
}

func prepareWatchCommand(ctx context.Context, tx pgx.Tx, sagaID, action, repo, correlationID string, occurredAt time.Time) error {
	eventID := uuid.NewString()
	eventType, err := commandEventType(action)
	if err != nil {
		return err
	}
	event := contracts.Envelope[contracts.RepoWatchCommandPayload]{
		EventID:       eventID,
		EventType:     eventType,
		OccurredAt:    occurredAt,
		Source:        "subscription-api-saga-orchestrator",
		CorrelationID: correlationID,
		Payload: contracts.RepoWatchCommandPayload{
			SagaID: sagaID,
			Action: action,
			Repo:   repo,
		},
	}
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal repo watch command: %w", err)
	}
	return outbox.InsertTx(ctx, tx, outbox.Event{
		EventID:       eventID,
		EventType:     event.EventType,
		OccurredAt:    event.OccurredAt,
		Source:        event.Source,
		CorrelationID: correlationID,
		Topic:         contracts.TopicWatchlistCommands,
		KafkaKey:      repo,
		Payload:       payload,
	})
}

func commandEventType(action string) (string, error) {
	switch action {
	case contracts.RepoWatchActionStart:
		return contracts.EventStartWatchingRepo, nil
	case contracts.RepoWatchActionStop:
		return contracts.EventStopWatchingRepo, nil
	default:
		return "", fmt.Errorf("unsupported repo watch action %q", action)
	}
}

func validateAction(action string) error {
	_, err := commandEventType(action)
	return err
}

func retryDelay(attempt int) time.Duration {
	if attempt <= 1 {
		return retryDelays[0]
	}
	if attempt >= len(retryDelays) {
		return retryDelays[len(retryDelays)-1]
	}
	return retryDelays[attempt-1]
}

func repoWatchAckEventType(action string) (string, error) {
	switch action {
	case contracts.RepoWatchActionStart:
		return contracts.EventRepoWatchStarted, nil
	case contracts.RepoWatchActionStop:
		return contracts.EventRepoWatchStopped, nil
	default:
		return "", fmt.Errorf("unsupported repo watch action %q", action)
	}
}

func loadSagaInstanceForUpdate(ctx context.Context, tx pgx.Tx, sagaID string) (Instance, error) {
	var instance Instance
	err := tx.QueryRow(ctx, `
SELECT saga_id::text, repo, action, status, attempts, correlation_id::text, deadline_at, last_error
FROM saga_instances
WHERE saga_id = $1
FOR UPDATE
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
		return Instance{}, err
	}
	return instance, nil
}
