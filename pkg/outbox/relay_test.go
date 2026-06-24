package outbox

import (
	"context"
	"errors"
	"testing"
	"time"

	"subber/pkg/logger"
)

type fakeOutboxStore struct {
	events       []Event
	fetchErr     error
	markPubErr   error
	markFailErr  error
	fetchLimit   int
	publishedIDs []string
	failedIDs    []string
	failedCauses []error
}

func (s *fakeOutboxStore) FetchUnpublished(_ context.Context, limit int) ([]Event, error) {
	s.fetchLimit = limit
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

type fakeLogger struct {
	warns  []string
	errors []string
	infos  []string
	fields []string
}

func (l *fakeLogger) WithField(key string, value any) logger.Logger {
	l.fields = append(l.fields, key)
	return l
}

func (l *fakeLogger) WithError(error) logger.Logger {
	return l
}

func (l *fakeLogger) Info(msg string) {
	l.infos = append(l.infos, msg)
}

func (l *fakeLogger) Warn(msg string) {
	l.warns = append(l.warns, msg)
}

func (l *fakeLogger) Error(msg string) {
	l.errors = append(l.errors, msg)
}

func (l *fakeLogger) Fatal(msg string) {
	l.errors = append(l.errors, msg)
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

func TestRelay_PublishFailureMarksFailedAndStopsBatch(t *testing.T) {
	publishErr := errors.New("kafka down")
	store := &fakeOutboxStore{events: []Event{
		{EventID: "1", Topic: "topic-a", KafkaKey: "key-a", Payload: []byte(`{"a":1}`)},
		{EventID: "2", Topic: "topic-b", KafkaKey: "key-b", Payload: []byte(`{"b":2}`)},
	}}
	log := &fakeLogger{}
	publisher := &fakeOutboxPublisher{err: publishErr}
	relay := NewRelayWithLogger(store, publisher, log, 10, time.Second)

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
	if len(publisher.messages) != 1 {
		t.Fatalf("published messages = %d, want 1", len(publisher.messages))
	}
	if len(log.warns) != 1 || log.warns[0] != "publish outbox event failed" {
		t.Fatalf("warn logs = %#v", log.warns)
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
