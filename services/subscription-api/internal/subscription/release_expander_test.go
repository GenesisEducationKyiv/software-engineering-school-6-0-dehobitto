package subscription

import (
	"context"
	"errors"
	"testing"

	"subber/pkg/contracts"
)

type fakeSubscriberStore struct {
	subscribers []string
	err         error
	repo        string
}

func (s *fakeSubscriberStore) GetSubscribers(_ context.Context, repo string) ([]string, error) {
	s.repo = repo
	return s.subscribers, s.err
}

type fakeReleasePublisher struct {
	err   error
	calls []releaseNotificationCall
}

type releaseNotificationCall struct {
	email         string
	repo          string
	tag           string
	correlationID string
}

func (p *fakeReleasePublisher) PublishReleaseNotification(_ context.Context, email, repo, tag, correlationID string) error {
	p.calls = append(p.calls, releaseNotificationCall{
		email:         email,
		repo:          repo,
		tag:           tag,
		correlationID: correlationID,
	})
	return p.err
}

func TestReleaseExpander_PublishesNotificationForEverySubscriber(t *testing.T) {
	store := &fakeSubscriberStore{subscribers: []string{"a@example.com", "b@example.com"}}
	publisher := &fakeReleasePublisher{}
	expander := NewReleaseExpander(store, publisher)

	event := contracts.Envelope[contracts.ReleaseDetectedPayload]{
		CorrelationID: "corr-1",
		Payload: contracts.ReleaseDetectedPayload{
			Repo: "owner/repo",
			Tag:  "v2.0.0",
		},
	}
	if err := expander.Expand(context.Background(), event); err != nil {
		t.Fatalf("Expand() error = %v", err)
	}

	if store.repo != "owner/repo" {
		t.Fatalf("store repo = %q, want owner/repo", store.repo)
	}
	if len(publisher.calls) != 2 {
		t.Fatalf("publisher calls = %d, want 2", len(publisher.calls))
	}
	if publisher.calls[0] != (releaseNotificationCall{"a@example.com", "owner/repo", "v2.0.0", "corr-1"}) {
		t.Fatalf("first call = %#v", publisher.calls[0])
	}
	if publisher.calls[1] != (releaseNotificationCall{"b@example.com", "owner/repo", "v2.0.0", "corr-1"}) {
		t.Fatalf("second call = %#v", publisher.calls[1])
	}
}

func TestReleaseExpander_NoSubscribersDoesNothing(t *testing.T) {
	publisher := &fakeReleasePublisher{}
	expander := NewReleaseExpander(&fakeSubscriberStore{}, publisher)

	err := expander.Expand(context.Background(), contracts.Envelope[contracts.ReleaseDetectedPayload]{
		Payload: contracts.ReleaseDetectedPayload{Repo: "owner/repo", Tag: "v1"},
	})
	if err != nil {
		t.Fatalf("Expand() error = %v", err)
	}
	if len(publisher.calls) != 0 {
		t.Fatalf("publisher calls = %d, want 0", len(publisher.calls))
	}
}

func TestReleaseExpander_ReturnsStoreAndPublisherErrors(t *testing.T) {
	storeErr := errors.New("db down")
	expander := NewReleaseExpander(&fakeSubscriberStore{err: storeErr}, &fakeReleasePublisher{})
	if err := expander.Expand(context.Background(), contracts.Envelope[contracts.ReleaseDetectedPayload]{}); !errors.Is(err, storeErr) {
		t.Fatalf("Expand() error = %v, want store error", err)
	}

	publisherErr := errors.New("outbox down")
	publisher := &fakeReleasePublisher{err: publisherErr}
	expander = NewReleaseExpander(&fakeSubscriberStore{subscribers: []string{"a@example.com"}}, publisher)
	err := expander.Expand(context.Background(), contracts.Envelope[contracts.ReleaseDetectedPayload]{
		Payload: contracts.ReleaseDetectedPayload{Repo: "owner/repo", Tag: "v1"},
	})
	if !errors.Is(err, publisherErr) {
		t.Fatalf("Expand() error = %v, want publisher error", err)
	}
}
