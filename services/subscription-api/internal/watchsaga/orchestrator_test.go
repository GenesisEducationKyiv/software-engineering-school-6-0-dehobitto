package watchsaga

import (
	"testing"
	"time"

	"subber/pkg/contracts"
)

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
