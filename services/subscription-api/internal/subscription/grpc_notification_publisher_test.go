package subscription

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	notificationv1 "subber/pkg/gen/notification/v1"
	"subber/pkg/requestid"
	"subber/services/subscription-api/internal/hash"
)

type fakeNotificationGRPCClient struct {
	req  *notificationv1.SendNotificationRequest
	err  error
	resp *notificationv1.SendNotificationResponse
}

func (c *fakeNotificationGRPCClient) SendNotification(_ context.Context, in *notificationv1.SendNotificationRequest, _ ...grpc.CallOption) (*notificationv1.SendNotificationResponse, error) {
	c.req = in
	if c.err != nil {
		return nil, c.err
	}
	if c.resp != nil {
		return c.resp, nil
	}
	return &notificationv1.SendNotificationResponse{Accepted: true}, nil
}

func TestGrpcNotificationPublisher_SendConfirmation(t *testing.T) {
	client := &fakeNotificationGRPCClient{}
	publisher := newGrpcNotificationPublisher(client, "http://localhost:8080", time.Second)
	ctx := requestid.WithContext(context.Background(), "11111111-1111-4111-8111-111111111111")

	if err := publisher.SendConfirmation(ctx, "USER@example.com", "owner/repo", "token-1"); err != nil {
		t.Fatalf("SendConfirmation() error = %v", err)
	}

	req := client.req
	if req == nil {
		t.Fatal("SendNotification was not called")
	}
	if req.GetNotificationId() == "" {
		t.Fatal("notification_id must be generated")
	}
	if req.GetCorrelationId() != "11111111-1111-4111-8111-111111111111" {
		t.Fatalf("CorrelationId = %q, want request correlation id", req.GetCorrelationId())
	}
	if req.GetRecipientEmail() != "USER@example.com" {
		t.Fatalf("RecipientEmail = %q", req.GetRecipientEmail())
	}
	if req.GetEmailHash() != hash.EmailHash("USER@example.com") {
		t.Fatalf("EmailHash = %q", req.GetEmailHash())
	}
	if !strings.Contains(req.GetMessage(), "http://localhost:8080/api/confirm/token-1") {
		t.Fatalf("message missing confirmation URL: %q", req.GetMessage())
	}
	if !strings.HasPrefix(req.GetIdempotencyKey(), "confirmation:owner/repo:"+req.GetEmailHash()+":") {
		t.Fatalf("IdempotencyKey = %q", req.GetIdempotencyKey())
	}
}

func TestGrpcNotificationPublisher_PublishReleaseNotification(t *testing.T) {
	client := &fakeNotificationGRPCClient{}
	publisher := newGrpcNotificationPublisher(client, "http://localhost:8080", time.Second)

	if err := publisher.PublishReleaseNotification(context.Background(), "user@example.com", "owner/repo", "v2.0.0", "corr-2"); err != nil {
		t.Fatalf("PublishReleaseNotification() error = %v", err)
	}

	req := client.req
	if req.GetCorrelationId() != "corr-2" || req.GetTag() != "v2.0.0" {
		t.Fatalf("unexpected release request: %#v", req)
	}
	if req.GetIdempotencyKey() != "owner/repo:v2.0.0:"+req.GetEmailHash() {
		t.Fatalf("IdempotencyKey = %q", req.GetIdempotencyKey())
	}
	if req.GetMessage() != "New release v2.0.0 for owner/repo!" {
		t.Fatalf("Message = %q", req.GetMessage())
	}
}

func TestGrpcNotificationPublisher_ReturnsClientErrors(t *testing.T) {
	clientErr := status.Error(codes.Unavailable, "notifier down")
	publisher := newGrpcNotificationPublisher(&fakeNotificationGRPCClient{err: clientErr}, "http://localhost:8080", time.Second)

	err := publisher.SendConfirmation(context.Background(), "user@example.com", "owner/repo", "token")
	if status.Code(err) != codes.Unavailable {
		t.Fatalf("status code = %v, want %v; err = %v", status.Code(err), codes.Unavailable, err)
	}
}

func TestGrpcNotificationPublisher_ReturnsNotAccepted(t *testing.T) {
	publisher := newGrpcNotificationPublisher(&fakeNotificationGRPCClient{
		resp: &notificationv1.SendNotificationResponse{Accepted: false},
	}, "http://localhost:8080", time.Second)

	err := publisher.SendConfirmation(context.Background(), "user@example.com", "owner/repo", "token")
	if err == nil || !strings.Contains(err.Error(), "not accepted") {
		t.Fatalf("SendConfirmation() error = %v, want not accepted", err)
	}
}

func TestGrpcNotificationPublisher_WrapsNonStatusErrors(t *testing.T) {
	publisher := newGrpcNotificationPublisher(&fakeNotificationGRPCClient{err: errors.New("boom")}, "http://localhost:8080", time.Second)

	err := publisher.SendConfirmation(context.Background(), "user@example.com", "owner/repo", "token")
	if err == nil || !strings.Contains(err.Error(), "send notification over grpc") {
		t.Fatalf("SendConfirmation() error = %v", err)
	}
}
