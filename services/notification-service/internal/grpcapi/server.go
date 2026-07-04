package grpcapi

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"subber/pkg/contracts"
	notificationv1 "subber/pkg/gen/notification/v1"
	"subber/pkg/requestid"
)

type NotificationProcessor interface {
	Process(ctx context.Context, payload contracts.NotificationSendRequestedPayload) error
}

type server struct {
	notificationv1.UnimplementedNotificationServiceServer
	processor NotificationProcessor
}

func NewServer(processor NotificationProcessor) notificationv1.NotificationServiceServer {
	return &server{processor: processor}
}

func (s *server) SendNotification(ctx context.Context, in *notificationv1.SendNotificationRequest) (*notificationv1.SendNotificationResponse, error) {
	if err := validateNotification(in); err != nil {
		return nil, err
	}

	if in.GetCorrelationId() != "" {
		ctx = requestid.WithContext(ctx, in.GetCorrelationId())
	}

	payload := buildNotification(in)

	if err := s.processor.Process(ctx, payload); err != nil {
		return nil, status.Errorf(codes.Internal, "process notification: %v", err)
	}

	return &notificationv1.SendNotificationResponse{
		Accepted: true,
	}, nil
}

func validateNotification(in *notificationv1.SendNotificationRequest) error {
	if in.GetNotificationId() == "" {
		return status.Error(codes.InvalidArgument, "notification_id is required")
	}
	if in.GetIdempotencyKey() == "" {
		return status.Error(codes.InvalidArgument, "idempotency_key is required")
	}
	if in.GetRecipientEmail() == "" {
		return status.Error(codes.InvalidArgument, "recipient_email is required")
	}
	if in.GetEmailHash() == "" {
		return status.Error(codes.InvalidArgument, "email_hash is required")
	}
	if in.GetRepo() == "" {
		return status.Error(codes.InvalidArgument, "repo is required")
	}
	if in.GetMessage() == "" {
		return status.Error(codes.InvalidArgument, "message is required")
	}
	return nil
}

func buildNotification(in *notificationv1.SendNotificationRequest) contracts.NotificationSendRequestedPayload {
	return contracts.NotificationSendRequestedPayload{
		NotificationID: in.GetNotificationId(),
		IdempotencyKey: in.GetIdempotencyKey(),
		RecipientEmail: in.GetRecipientEmail(),
		EmailHash:      in.GetEmailHash(),
		Repo:           in.GetRepo(),
		Tag:            in.GetTag(),
		Message:        in.GetMessage(),
	}
}
