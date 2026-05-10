package service

import (
	"context"

	"subber/internal/models"
)

// SubscriptionRepository is the persistence contract required by SubscriptionService.
type SubscriptionRepository interface {
	SubscriptionExists(ctx context.Context, email, repo string) (bool, error)
	SaveSubscription(ctx context.Context, sub models.Subscription) error
}
