package grpcapi

import (
	"context"
	"errors"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"subber/pkg/contracts"
	notificationv1 "subber/pkg/gen/notification/v1"
)

type fakeProcessor struct {
	payload contracts.NotificationSendRequestedPayload
	err     error
	called  bool
}

func (p *fakeProcessor) Process(_ context.Context, payload contracts.NotificationSendRequestedPayload) error {
	p.called = true
	p.payload = payload
	return p.err
}

func validRequest() *notificationv1.SendNotificationRequest {
	return &notificationv1.SendNotificationRequest{
		NotificationId: "notification-1",
		IdempotencyKey: "confirmation:owner/repo:hash",
		RecipientEmail: "user@example.com",
		EmailHash:      "email-hash",
		Repo:           "owner/repo",
		Message:        "Please confirm",
		CorrelationId:  "correlation-1",
	}
}

func TestSendNotification_ValidRequestProcessesPayload(t *testing.T) {
	processor := &fakeProcessor{}
	srv := NewServer(processor)

	resp, err := srv.SendNotification(context.Background(), validRequest())
	if err != nil {
		t.Fatalf("SendNotification() error = %v", err)
	}
	if !resp.GetAccepted() {
		t.Fatal("Accepted = false, want true")
	}
	if !processor.called {
		t.Fatal("processor was not called")
	}
	if processor.payload.NotificationID != "notification-1" ||
		processor.payload.IdempotencyKey != "confirmation:owner/repo:hash" ||
		processor.payload.RecipientEmail != "user@example.com" ||
		processor.payload.EmailHash != "email-hash" ||
		processor.payload.Repo != "owner/repo" ||
		processor.payload.Message != "Please confirm" {
		t.Fatalf("unexpected payload: %#v", processor.payload)
	}
}

func TestSendNotification_InvalidRequests(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*notificationv1.SendNotificationRequest)
	}{
		{"notification_id", func(req *notificationv1.SendNotificationRequest) { req.NotificationId = "" }},
		{"idempotency_key", func(req *notificationv1.SendNotificationRequest) { req.IdempotencyKey = "" }},
		{"recipient_email", func(req *notificationv1.SendNotificationRequest) { req.RecipientEmail = "" }},
		{"email_hash", func(req *notificationv1.SendNotificationRequest) { req.EmailHash = "" }},
		{"repo", func(req *notificationv1.SendNotificationRequest) { req.Repo = "" }},
		{"message", func(req *notificationv1.SendNotificationRequest) { req.Message = "" }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := validRequest()
			tt.mutate(req)
			processor := &fakeProcessor{}
			_, err := NewServer(processor).SendNotification(context.Background(), req)
			if status.Code(err) != codes.InvalidArgument {
				t.Fatalf("status code = %v, want %v; err = %v", status.Code(err), codes.InvalidArgument, err)
			}
			if processor.called {
				t.Fatal("processor must not be called for invalid request")
			}
		})
	}
}

func TestSendNotification_ProcessError(t *testing.T) {
	processorErr := errors.New("smtp down")
	_, err := NewServer(&fakeProcessor{err: processorErr}).SendNotification(context.Background(), validRequest())
	if status.Code(err) != codes.Internal {
		t.Fatalf("status code = %v, want %v; err = %v", status.Code(err), codes.Internal, err)
	}
}
