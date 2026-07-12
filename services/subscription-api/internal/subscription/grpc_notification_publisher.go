package subscription

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"

	"subber/pkg/contracts"
	notificationv1 "subber/pkg/gen/notification/v1"
	"subber/services/subscription-api/internal/hash"
)

const defaultNotificationGRPCTimeout = 5 * time.Second

type notificationGRPCClient interface {
	SendNotification(ctx context.Context, in *notificationv1.SendNotificationRequest, opts ...grpc.CallOption) (*emptypb.Empty, error)
}

type GrpcNotificationPublisher struct {
	client  notificationGRPCClient
	baseURL string
	timeout time.Duration
}

func NewGrpcNotificationPublisher(conn grpc.ClientConnInterface, baseURL string) *GrpcNotificationPublisher {
	return newGrpcNotificationPublisher(notificationv1.NewNotificationServiceClient(conn), baseURL, defaultNotificationGRPCTimeout)
}

func newGrpcNotificationPublisher(client notificationGRPCClient, baseURL string, timeout time.Duration) *GrpcNotificationPublisher {
	if timeout <= 0 {
		timeout = defaultNotificationGRPCTimeout
	}
	return &GrpcNotificationPublisher{client: client, baseURL: baseURL, timeout: timeout}
}

func (p *GrpcNotificationPublisher) SendConfirmation(ctx context.Context, email, repo, token string) error {
	notificationID := uuid.NewString()
	correlationID := correlationIDFromRequest(ctx, notificationID)
	confirmURL := fmt.Sprintf("%s/api/confirm/%s", p.baseURL, token)
	message := fmt.Sprintf("Welcome! Please confirm your subscription to GitHub repository updates by clicking here: %s", confirmURL)
	emailHash := hash.EmailHash(email)
	payload := contracts.NotificationSendRequestedPayload{
		NotificationID: notificationID,
		IdempotencyKey: fmt.Sprintf("confirmation:%s:%s:%s", repo, emailHash, hash.TextHash(message)),
		RecipientEmail: email,
		EmailHash:      emailHash,
		Repo:           repo,
		Message:        message,
	}
	return p.send(ctx, payload, correlationID)
}

func (p *GrpcNotificationPublisher) PublishReleaseNotification(ctx context.Context, email, repo, tag, correlationID string) error {
	notificationID := uuid.NewString()
	event, _, err := buildReleaseNotification(email, repo, tag, notificationID, correlationID, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("build release notification: %w", err)
	}
	return p.send(ctx, event.Payload, event.CorrelationID)
}

func (p *GrpcNotificationPublisher) send(ctx context.Context, payload contracts.NotificationSendRequestedPayload, correlationID string) error {
	callCtx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()

	_, err := p.client.SendNotification(callCtx, &notificationv1.SendNotificationRequest{
		NotificationId: payload.NotificationID,
		IdempotencyKey: payload.IdempotencyKey,
		RecipientEmail: payload.RecipientEmail,
		EmailHash:      payload.EmailHash,
		Repo:           payload.Repo,
		Tag:            payload.Tag,
		Message:        payload.Message,
		CorrelationId:  correlationID,
	})
	if err != nil {
		return fmt.Errorf("send notification over grpc: %w", err)
	}
	return nil
}
