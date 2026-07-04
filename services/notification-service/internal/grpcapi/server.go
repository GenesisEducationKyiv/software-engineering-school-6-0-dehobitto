package grpcapi

import (
	"context"
	"fmt"

	"buf.build/go/protovalidate"
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
	validator protovalidate.Validator
}

func NewServer(processor NotificationProcessor) notificationv1.NotificationServiceServer {
	validator, err := protovalidate.New()
	if err != nil {
		panic(fmt.Sprintf("create notification validator: %v", err))
	}

	return &server{
		processor: processor,
		validator: validator,
	}
}

func (s *server) SendNotification(ctx context.Context, in *notificationv1.SendNotificationRequest) (*notificationv1.SendNotificationResponse, error) {
	if err := s.validator.Validate(in); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid notification request: %v", err)
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
