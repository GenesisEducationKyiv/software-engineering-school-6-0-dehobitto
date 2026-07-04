package subscription

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
)

type fakeStore struct {
	exists     bool
	existsErr  error
	saveErr    error
	saved      Subscription
	saveCalled bool
}

func (s *fakeStore) SubscriptionExists(context.Context, string, string) (bool, error) {
	return s.exists, s.existsErr
}

func (s *fakeStore) SaveSubscription(_ context.Context, sub Subscription) error {
	s.saveCalled = true
	s.saved = sub
	return s.saveErr
}

func (s *fakeStore) SaveSubscriptionWithConfirmation(ctx context.Context, sub Subscription, publisher NotificationPublisher) error {
	if err := s.SaveSubscription(ctx, sub); err != nil {
		return err
	}
	return publisher.PublishConfirmationTx(ctx, nil, sub.Email, sub.Repo, sub.Token)
}

type fakeGitHub struct {
	existsErr error
	tag       string
	tagErr    error
	checked   bool
	tagCalled bool
}

func (g *fakeGitHub) CheckIfRepoExists(context.Context, string) error {
	g.checked = true
	return g.existsErr
}

func (g *fakeGitHub) GetLatestTag(context.Context, string) (string, error) {
	g.tagCalled = true
	return g.tag, g.tagErr
}

type fakeNotifications struct {
	called bool
	email  string
	repo   string
	token  string
	err    error
}

func (n *fakeNotifications) PublishConfirmationTx(_ context.Context, _ pgx.Tx, email, repo, token string) error {
	n.called = true
	n.email = email
	n.repo = repo
	n.token = token
	return n.err
}

func TestSubscribe_SuccessSavesUnconfirmedAndPublishesConfirmation(t *testing.T) {
	store := &fakeStore{}
	gh := &fakeGitHub{tag: "v1.0.0"}
	notifications := &fakeNotifications{}
	svc := NewService(store, notifications, gh, nil)

	if err := svc.Subscribe(context.Background(), "user@example.com", "owner/repo"); err != nil {
		t.Fatalf("Subscribe() error = %v", err)
	}

	if !store.saveCalled {
		t.Fatal("expected subscription to be saved")
	}
	if store.saved.Confirmed {
		t.Fatal("subscription must start unconfirmed")
	}
	if store.saved.LastSeenTag != "v1.0.0" {
		t.Fatalf("LastSeenTag = %q, want v1.0.0", store.saved.LastSeenTag)
	}
	if !notifications.called {
		t.Fatal("expected confirmation notification")
	}
	if notifications.token == "" || notifications.token != store.saved.Token {
		t.Fatalf("confirmation token = %q, saved token = %q", notifications.token, store.saved.Token)
	}
}

func TestSubscribe_AlreadySubscribedShortCircuits(t *testing.T) {
	store := &fakeStore{exists: true}
	gh := &fakeGitHub{}
	notifications := &fakeNotifications{}
	svc := NewService(store, notifications, gh, nil)

	err := svc.Subscribe(context.Background(), "user@example.com", "owner/repo")
	if !errors.Is(err, ErrAlreadySubscribed) {
		t.Fatalf("Subscribe() error = %v, want ErrAlreadySubscribed", err)
	}
	if gh.checked || store.saveCalled || notifications.called {
		t.Fatal("duplicate subscription should not validate repo, save, or publish notification")
	}
}

func TestSubscribe_RepositoryCheckFailurePropagates(t *testing.T) {
	store := &fakeStore{existsErr: errors.New("db timeout")}
	gh := &fakeGitHub{}
	notifications := &fakeNotifications{}
	svc := NewService(store, notifications, gh, nil)

	if err := svc.Subscribe(context.Background(), "user@example.com", "owner/repo"); err == nil {
		t.Fatal("expected repository error, got nil")
	}
	if gh.checked || store.saveCalled || notifications.called {
		t.Fatal("repository check failure should not validate repo, save, or publish notification")
	}
}

func TestSubscribe_MapsGitHubErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want error
	}{
		{"not found", ErrGitHubNotFound, ErrRepoNotFound},
		{"rate limit", ErrGitHubAPILimit, ErrGitHubRateLimit},
		{"unavailable", errors.New("network"), ErrGitHubUnavailable},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewService(&fakeStore{}, &fakeNotifications{}, &fakeGitHub{existsErr: tt.err}, nil)
			err := svc.Subscribe(context.Background(), "user@example.com", "owner/repo")
			if !errors.Is(err, tt.want) {
				t.Fatalf("Subscribe() error = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestSubscribe_SaveFailureDoesNotPublishConfirmation(t *testing.T) {
	store := &fakeStore{saveErr: errors.New("db down")}
	notifications := &fakeNotifications{}
	svc := NewService(store, notifications, &fakeGitHub{tag: "v1.0.0"}, nil)

	if err := svc.Subscribe(context.Background(), "user@example.com", "owner/repo"); err == nil {
		t.Fatal("expected save error, got nil")
	}
	if !store.saveCalled {
		t.Fatal("expected save attempt")
	}
	if notifications.called {
		t.Fatal("confirmation must not be published when subscription was not saved")
	}
}

func TestSubscribe_NotificationFailurePropagatesAfterSave(t *testing.T) {
	store := &fakeStore{}
	notifications := &fakeNotifications{err: errors.New("outbox down")}
	svc := NewService(store, notifications, &fakeGitHub{tag: "v1.0.0"}, nil)

	if err := svc.Subscribe(context.Background(), "user@example.com", "owner/repo"); err == nil {
		t.Fatal("expected notification publisher error, got nil")
	}
	if !store.saveCalled || !notifications.called {
		t.Fatal("expected saved subscription and attempted notification")
	}
}

func TestSubscribe_TagFetchFailureStillSaves(t *testing.T) {
	store := &fakeStore{}
	notifications := &fakeNotifications{}
	svc := NewService(store, notifications, &fakeGitHub{tagErr: errors.New("timeout")}, nil)

	if err := svc.Subscribe(context.Background(), "user@example.com", "owner/repo"); err != nil {
		t.Fatalf("Subscribe() error = %v", err)
	}
	if !store.saveCalled {
		t.Fatal("expected save even when initial tag fetch fails")
	}
	if store.saved.LastSeenTag != "" {
		t.Fatalf("LastSeenTag = %q, want empty", store.saved.LastSeenTag)
	}
}
