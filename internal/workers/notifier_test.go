package workers

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"

	"subber/internal/models"
)

type mockEmailSender struct {
	mock.Mock
}

func (m *mockEmailSender) Send(to, message string) error {
	return m.Called(to, message).Error(0)
}

func TestNotifier_SendsEmail(t *testing.T) {
	sender := new(mockEmailSender)
	sender.On("Send", "user@example.com", "hello").Return(nil).Once()
	jobs := make(chan models.NotificationJob, 1)
	jobs <- models.NotificationJob{Email: "user@example.com", Message: "hello"}
	close(jobs)

	err := NewNotifierWorker(sender).Start(context.Background(), jobs)
	sender.AssertExpectations(t)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNotifier_ContinuesAfterSendFailure(t *testing.T) {
	sender := new(mockEmailSender)
	sender.On("Send", "a@example.com", "msg").Return(errors.New("smtp error")).Once()
	sender.On("Send", "b@example.com", "msg").Return(errors.New("smtp error")).Once()
	jobs := make(chan models.NotificationJob, 2)
	jobs <- models.NotificationJob{Email: "a@example.com", Message: "msg"}
	jobs <- models.NotificationJob{Email: "b@example.com", Message: "msg"}
	close(jobs)

	err := NewNotifierWorker(sender).Start(context.Background(), jobs)
	sender.AssertExpectations(t)

	if err != nil {
		t.Fatalf("worker stopped on error: %v", err)
	}
}

func TestNotifier_StopsOnContextCancel(t *testing.T) {
	sender := new(mockEmailSender)
	jobs := make(chan models.NotificationJob)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := NewNotifierWorker(sender).Start(ctx, jobs)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	sender.AssertNotCalled(t, "Send", mock.Anything, mock.Anything)
}
