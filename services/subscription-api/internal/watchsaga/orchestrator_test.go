package watchsaga

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"subber/pkg/contracts"
)

func TestHandleRequestCreatesSagaAndCommandWithoutRealDB(t *testing.T) {
	t.Parallel()

	tx := &mockTx{
		execTags: []pgconn.CommandTag{
			pgconn.NewCommandTag("INSERT 0 1"),
			pgconn.NewCommandTag("INSERT 0 1"),
		},
	}
	orchestrator := NewWithTransactions(mockTransactions{tx: tx})
	orchestrator.now = func() time.Time { return time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC) }

	err := orchestrator.HandleRequest(context.Background(), contracts.Envelope[contracts.RepoWatchSagaPayload]{
		EventType:     contracts.EventRepoWatchSagaRequested,
		CorrelationID: "00000000-0000-0000-0000-000000000002",
		Payload: contracts.RepoWatchSagaPayload{
			SagaID: "00000000-0000-0000-0000-000000000001",
			Action: contracts.RepoWatchActionStart,
			Repo:   "owner/repo",
		},
	})
	if err != nil {
		t.Fatalf("HandleRequest() error = %v", err)
	}
	if !tx.committed {
		t.Fatal("transaction was not committed")
	}
	if len(tx.execCalls) != 2 {
		t.Fatalf("Exec calls = %d, want saga insert and outbox insert", len(tx.execCalls))
	}
	if !strings.Contains(tx.execCalls[0].sql, "INSERT INTO saga_instances") {
		t.Fatalf("first Exec SQL = %q", tx.execCalls[0].sql)
	}
	if !strings.Contains(tx.execCalls[1].sql, "INSERT INTO outbox_events") {
		t.Fatalf("second Exec SQL = %q", tx.execCalls[1].sql)
	}
	if tx.execCalls[1].args[5] != contracts.TopicWatchlistCommands {
		t.Fatalf("outbox topic = %#v, want %s", tx.execCalls[1].args[5], contracts.TopicWatchlistCommands)
	}
}

func TestHandleAckCompletesSagaWithoutRealDB(t *testing.T) {
	t.Parallel()

	sagaID := "00000000-0000-0000-0000-000000000001"
	tx := &mockTx{
		row: mockRow{values: []any{
			sagaID,
			"owner/repo",
			contracts.RepoWatchActionStart,
			StatusCommandSent,
			1,
			"00000000-0000-0000-0000-000000000002",
			time.Date(2026, 6, 26, 12, 0, 0, 0, time.UTC),
			(*string)(nil),
		}},
		execTags: []pgconn.CommandTag{pgconn.NewCommandTag("UPDATE 1")},
	}
	orchestrator := NewWithTransactions(mockTransactions{tx: tx})

	err := orchestrator.HandleAck(context.Background(), contracts.Envelope[contracts.RepoWatchAckPayload]{
		EventType: contracts.EventRepoWatchStarted,
		Payload: contracts.RepoWatchAckPayload{
			SagaID: sagaID,
			Action: contracts.RepoWatchActionStart,
			Repo:   "owner/repo",
		},
	})
	if err != nil {
		t.Fatalf("HandleAck() error = %v", err)
	}
	if !tx.committed {
		t.Fatal("transaction was not committed")
	}
	if len(tx.execCalls) != 1 {
		t.Fatalf("Exec calls = %d, want complete saga update", len(tx.execCalls))
	}
	if tx.execCalls[0].args[1] != StatusCompleted {
		t.Fatalf("status arg = %#v, want %s", tx.execCalls[0].args[1], StatusCompleted)
	}
}

func TestHandleAckRejectsPayloadMismatchBeforeUpdate(t *testing.T) {
	t.Parallel()

	sagaID := "00000000-0000-0000-0000-000000000001"
	tx := &mockTx{
		row: mockRow{values: []any{
			sagaID,
			"owner/repo",
			contracts.RepoWatchActionStart,
			StatusCommandSent,
			1,
			"00000000-0000-0000-0000-000000000002",
			time.Date(2026, 6, 26, 12, 0, 0, 0, time.UTC),
			(*string)(nil),
		}},
	}
	orchestrator := NewWithTransactions(mockTransactions{tx: tx})

	err := orchestrator.HandleAck(context.Background(), contracts.Envelope[contracts.RepoWatchAckPayload]{
		EventType: contracts.EventRepoWatchStarted,
		Payload: contracts.RepoWatchAckPayload{
			SagaID: sagaID,
			Action: contracts.RepoWatchActionStart,
			Repo:   "other/repo",
		},
	})
	if err == nil {
		t.Fatal("HandleAck() error = nil, want payload mismatch")
	}
	if tx.committed {
		t.Fatal("transaction was committed on mismatch")
	}
	if len(tx.execCalls) != 0 {
		t.Fatalf("Exec calls = %d, want no update", len(tx.execCalls))
	}
}

func TestRetryDelay(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		attempt int
		want    time.Duration
	}{
		{name: "zero", attempt: 0, want: time.Minute},
		{name: "first", attempt: 1, want: time.Minute},
		{name: "second", attempt: 2, want: 5 * time.Minute},
		{name: "third", attempt: 3, want: 15 * time.Minute},
		{name: "fourth", attempt: 4, want: time.Hour},
		{name: "overflow", attempt: 99, want: time.Hour},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := retryDelay(tt.attempt); got != tt.want {
				t.Fatalf("retryDelay(%d) = %v, want %v", tt.attempt, got, tt.want)
			}
		})
	}
}

func TestCommandEventType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		action  string
		want    string
		wantErr bool
	}{
		{name: "start", action: contracts.RepoWatchActionStart, want: contracts.EventStartWatchingRepo},
		{name: "stop", action: contracts.RepoWatchActionStop, want: contracts.EventStopWatchingRepo},
		{name: "invalid", action: "pause", wantErr: true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := commandEventType(tt.action)
			if tt.wantErr {
				if err == nil {
					t.Fatal("commandEventType() error = nil, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("commandEventType() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("commandEventType() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRepoWatchAckEventType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		action  string
		want    string
		wantErr bool
	}{
		{name: "start", action: contracts.RepoWatchActionStart, want: contracts.EventRepoWatchStarted},
		{name: "stop", action: contracts.RepoWatchActionStop, want: contracts.EventRepoWatchStopped},
		{name: "invalid", action: "pause", wantErr: true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := repoWatchAckEventType(tt.action)
			if tt.wantErr {
				if err == nil {
					t.Fatal("repoWatchAckEventType() error = nil, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("repoWatchAckEventType() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("repoWatchAckEventType() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestValidateAction(t *testing.T) {
	t.Parallel()

	if err := validateAction(contracts.RepoWatchActionStart); err != nil {
		t.Fatalf("validateAction(start) error = %v", err)
	}
	if err := validateAction("pause"); err == nil {
		t.Fatal("validateAction(invalid) error = nil, want error")
	}
}

type mockTransactions struct {
	tx  pgx.Tx
	err error
}

func (m mockTransactions) BeginTx(context.Context) (pgx.Tx, error) {
	return m.tx, m.err
}

type mockExecCall struct {
	sql  string
	args []any
}

type mockTx struct {
	pgx.Tx
	row        pgx.Row
	execTags   []pgconn.CommandTag
	execCalls  []mockExecCall
	committed  bool
	rolledBack bool
}

func (t *mockTx) Begin(context.Context) (pgx.Tx, error) {
	return nil, errors.New("nested transactions are not supported in mockTx")
}

func (t *mockTx) Commit(context.Context) error {
	t.committed = true
	return nil
}

func (t *mockTx) Rollback(context.Context) error {
	t.rolledBack = true
	return nil
}

func (t *mockTx) Exec(_ context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
	t.execCalls = append(t.execCalls, mockExecCall{sql: sql, args: arguments})
	if len(t.execTags) == 0 {
		return pgconn.NewCommandTag("UPDATE 1"), nil
	}
	tag := t.execTags[0]
	t.execTags = t.execTags[1:]
	return tag, nil
}

func (t *mockTx) QueryRow(context.Context, string, ...any) pgx.Row {
	return t.row
}

type mockRow struct {
	values []any
	err    error
}

func (r mockRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	if len(dest) != len(r.values) {
		return errors.New("mock row destination count mismatch")
	}
	for i, value := range r.values {
		switch target := dest[i].(type) {
		case *string:
			*target = value.(string)
		case *int:
			*target = value.(int)
		case *time.Time:
			*target = value.(time.Time)
		case **string:
			if value == nil {
				*target = nil
				continue
			}
			*target = value.(*string)
		default:
			return errors.New("unsupported mock row destination")
		}
	}
	return nil
}
