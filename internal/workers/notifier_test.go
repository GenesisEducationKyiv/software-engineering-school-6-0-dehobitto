package workers

import (
	"context"
	"errors"
	"testing"
)

type fakeSender struct {
	err   error
	calls []string
}

func (f *fakeSender) Send(to, _ string) error {
	f.calls = append(f.calls, to)
	return f.err
}

func TestNotifier_SendsEmail(t *testing.T) {
	sender := &fakeSender{}
	jobs := make(chan NotificationJob, 1)
	jobs <- NotificationJob{Email: "user@example.com", Message: "hello"}
	close(jobs)

	err := NewNotifierWorker(sender).Start(context.Background(), jobs)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sender.calls) != 1 || sender.calls[0] != "user@example.com" {
		t.Errorf("calls = %v, want [user@example.com]", sender.calls)
	}
}

func TestNotifier_ContinuesAfterSendFailure(t *testing.T) {
	sender := &fakeSender{err: errors.New("smtp error")}
	jobs := make(chan NotificationJob, 2)
	jobs <- NotificationJob{Email: "a@example.com", Message: "msg"}
	jobs <- NotificationJob{Email: "b@example.com", Message: "msg"}
	close(jobs)

	err := NewNotifierWorker(sender).Start(context.Background(), jobs)

	if err != nil {
		t.Fatalf("worker stopped on error: %v", err)
	}
	if len(sender.calls) != 2 {
		t.Errorf("calls = %d, want 2 (worker must not stop on send failure)", len(sender.calls))
	}
}

func TestNotifier_StopsOnContextCancel(t *testing.T) {
	sender := &fakeSender{}
	jobs := make(chan NotificationJob)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := NewNotifierWorker(sender).Start(ctx, jobs)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
