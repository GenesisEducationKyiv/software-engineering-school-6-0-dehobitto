package outbox

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakeOutboxStore struct {
	events       []Event
	fetchErr     error
	fetchErrs    []error
	markPubErr   error
	markFailErr  error
	fetchLimit   int
	publishedIDs []string
	failedIDs    []string
	failedCauses []error
}

func (s *fakeOutboxStore) FetchUnpublished(_ context.Context, limit int) ([]Event, error) {
	s.fetchLimit = limit
	if len(s.fetchErrs) > 0 {
		err := s.fetchErrs[0]
		s.fetchErrs = s.fetchErrs[1:]
		return s.events, err
	}
	return s.events, s.fetchErr
}

func (s *fakeOutboxStore) MarkPublished(_ context.Context, eventID string) error {
	s.publishedIDs = append(s.publishedIDs, eventID)
	return s.markPubErr
}

func (s *fakeOutboxStore) MarkFailed(_ context.Context, eventID string, cause error) error {
	s.failedIDs = append(s.failedIDs, eventID)
	s.failedCauses = append(s.failedCauses, cause)
	return s.markFailErr
}

type fakeOutboxPublisher struct {
	err      error
	messages []publishedMessage
}

type publishedMessage struct {
	topic string
	key   string
	value string
}

func (p *fakeOutboxPublisher) Publish(_ context.Context, topic, key string, value []byte) error {
	p.messages = append(p.messages, publishedMessage{topic: topic, key: key, value: string(value)})
	return p.err
}

func TestRelay_PublishOncePublishesAndMarksEvents(t *testing.T) {
	store := &fakeOutboxStore{events: []Event{
		{EventID: "1", Topic: "topic-a", KafkaKey: "key-a", Payload: []byte(`{"a":1}`)},
		{EventID: "2", Topic: "topic-b", KafkaKey: "key-b", Payload: []byte(`{"b":2}`)},
	}}
	publisher := &fakeOutboxPublisher{}
	relay := NewRelay(store, publisher, 10, time.Second)

	if err := relay.PublishOnce(context.Background()); err != nil {
		t.Fatalf("PublishOnce() error = %v", err)
	}
	if store.fetchLimit != 10 {
		t.Fatalf("fetch limit = %d, want 10", store.fetchLimit)
	}
	if len(publisher.messages) != 2 {
		t.Fatalf("published messages = %d, want 2", len(publisher.messages))
	}
	if store.publishedIDs[0] != "1" || store.publishedIDs[1] != "2" {
		t.Fatalf("published ids = %#v", store.publishedIDs)
	}
}

func TestRelay_PublishFailureMarksFailedAndContinues(t *testing.T) {
	publishErr := errors.New("kafka down")
	store := &fakeOutboxStore{events: []Event{
		{EventID: "1", Topic: "topic-a", KafkaKey: "key-a", Payload: []byte(`{"a":1}`)},
	}}
	relay := NewRelay(store, &fakeOutboxPublisher{err: publishErr}, 10, time.Second)

	if err := relay.PublishOnce(context.Background()); err != nil {
		t.Fatalf("PublishOnce() error = %v", err)
	}
	if len(store.failedIDs) != 1 || store.failedIDs[0] != "1" {
		t.Fatalf("failed ids = %#v", store.failedIDs)
	}
	if !errors.Is(store.failedCauses[0], publishErr) {
		t.Fatalf("failed cause = %v, want publish err", store.failedCauses[0])
	}
	if len(store.publishedIDs) != 0 {
		t.Fatalf("published ids = %#v, want none", store.publishedIDs)
	}
}

func TestRelay_ReturnsStoreErrors(t *testing.T) {
	fetchErr := errors.New("db fetch down")
	if err := NewRelay(&fakeOutboxStore{fetchErr: fetchErr}, &fakeOutboxPublisher{}, 10, time.Second).PublishOnce(context.Background()); !errors.Is(err, fetchErr) {
		t.Fatalf("fetch error = %v, want %v", err, fetchErr)
	}

	markPublishedErr := errors.New("mark published down")
	err := NewRelay(&fakeOutboxStore{
		events:     []Event{{EventID: "1", Topic: "topic", KafkaKey: "key", Payload: []byte(`{}`)}},
		markPubErr: markPublishedErr,
	}, &fakeOutboxPublisher{}, 10, time.Second).PublishOnce(context.Background())
	if !errors.Is(err, markPublishedErr) {
		t.Fatalf("mark published error = %v, want %v", err, markPublishedErr)
	}

	markFailedErr := errors.New("mark failed down")
	err = NewRelay(&fakeOutboxStore{
		events:      []Event{{EventID: "1", Topic: "topic", KafkaKey: "key", Payload: []byte(`{}`)}},
		markFailErr: markFailedErr,
	}, &fakeOutboxPublisher{err: errors.New("kafka down")}, 10, time.Second).PublishOnce(context.Background())
	if !errors.Is(err, markFailedErr) {
		t.Fatalf("mark failed error = %v, want %v", err, markFailedErr)
	}
}

func TestRelay_DefaultsBatchSizeAndInterval(t *testing.T) {
	relay := NewRelay(&fakeOutboxStore{}, &fakeOutboxPublisher{}, 0, 0)
	if relay.batchSize != 100 {
		t.Fatalf("batchSize = %d, want 100", relay.batchSize)
	}
	if relay.interval != time.Second {
		t.Fatalf("interval = %s, want 1s", relay.interval)
	}
}

func TestRelay_StartContinuesAfterTransientFetchError(t *testing.T) {
	store := &fakeOutboxStore{
		events: []Event{{EventID: "1", Topic: "topic-a", KafkaKey: "key-a", Payload: []byte(`{"a":1}`)}},
		fetchErrs: []error{
			errors.New("temporary db error"),
			nil,
		},
	}
	publisher := &fakeOutboxPublisher{}
	relay := NewRelay(store, publisher, 10, 5*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- relay.Start(ctx)
	}()

	deadline := time.Now().Add(200 * time.Millisecond)
	for len(store.publishedIDs) == 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	cancel()

	if err := <-done; err != nil {
		t.Fatalf("Start() error = %v, want nil", err)
	}
	if len(store.publishedIDs) != 1 || store.publishedIDs[0] != "1" {
		t.Fatalf("published ids = %#v, want [\"1\"]", store.publishedIDs)
	}
}
