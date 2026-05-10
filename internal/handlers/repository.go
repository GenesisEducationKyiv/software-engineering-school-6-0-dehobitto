package handlers

import (
	"context"

	"subber/internal/models"
)

// SubscriptionRepository is the persistence contract required by Handler.
type SubscriptionRepository interface {
	ConfirmSubscriptionByToken(ctx context.Context, token string) error
	Unsubscribe(ctx context.Context, token string) error
	GetSubscriptions(ctx context.Context, email string) ([]models.Subscription, error)
}
