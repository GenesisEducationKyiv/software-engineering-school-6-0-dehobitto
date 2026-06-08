package logger

import (
	"encoding/json"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
)

type fakeLogPublisher struct {
	mu        sync.Mutex
	published [][]byte
	err       error
	block     chan struct{}
	started   chan struct{}
}

type fakeLogMetrics struct {
	enqueued  atomic.Uint64
	dropped   atomic.Uint64
	published atomic.Uint64
	errors    atomic.Uint64
}

func (m *fakeLogMetrics) IncLogEntriesEnqueued()  { m.enqueued.Add(1) }
func (m *fakeLogMetrics) IncLogEntriesDropped()   { m.dropped.Add(1) }
func (m *fakeLogMetrics) IncLogEntriesPublished() { m.published.Add(1) }
func (m *fakeLogMetrics) IncLogPublishErrors()    { m.errors.Add(1) }

func (p *fakeLogPublisher) Publish(body []byte) error {
	if p.started != nil {
		select {
		case p.started <- struct{}{}:
		default:
		}
	}
	if p.block != nil {
		<-p.block
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	p.published = append(p.published, append([]byte(nil), body...))
	return p.err
}

func (p *fakeLogPublisher) count() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.published)
}

func (p *fakeLogPublisher) last() []byte {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]byte(nil), p.published[len(p.published)-1]...)
}

func TestRabbitMQHookPublishesLogEntryAsync(t *testing.T) {
	publisher := &fakeLogPublisher{}
	hook := newRabbitMQHook(publisher, 10, nil)

	log := logrus.New()
	log.SetFormatter(&logrus.JSONFormatter{})
	entry := logrus.NewEntry(log).
		WithField("component", "test").
		WithField("request_id", "req-1")
	if err := hook.Fire(entry); err != nil {
		t.Fatalf("Fire returned error: %v", err)
	}
	hook.Close()

	if publisher.count() != 1 {
		t.Fatalf("published = %d, want 1", publisher.count())
	}

	var payload map[string]any
	if err := json.Unmarshal(publisher.last(), &payload); err != nil {
		t.Fatalf("published payload is not JSON: %v", err)
	}
	if payload["component"] != "test" {
		t.Fatalf("component = %v, want test", payload["component"])
	}
	if payload["request_id"] != "req-1" {
		t.Fatalf("request_id = %v, want req-1", payload["request_id"])
	}
}

func TestRabbitMQHookDropsWhenBufferIsFull(t *testing.T) {
	publisher := &fakeLogPublisher{
		block:   make(chan struct{}),
		started: make(chan struct{}, 1),
	}
	hook := newRabbitMQHook(publisher, 1, nil)

	entry := logrus.NewEntry(logrus.New())
	if err := hook.Fire(entry); err != nil {
		t.Fatalf("first Fire returned error: %v", err)
	}

	select {
	case <-publisher.started:
	case <-time.After(time.Second):
		t.Fatal("publisher did not start")
	}

	if err := hook.Fire(entry); err != nil {
		t.Fatalf("second Fire returned error: %v", err)
	}
	if err := hook.Fire(entry); err != nil {
		t.Fatalf("third Fire returned error: %v", err)
	}

	if dropped := hook.dropped.Load(); dropped != 1 {
		t.Fatalf("dropped = %d, want 1", dropped)
	}

	close(publisher.block)
	hook.Close()
}

func TestRabbitMQHookCloseDrainsBufferedEntries(t *testing.T) {
	publisher := &fakeLogPublisher{}
	hook := newRabbitMQHook(publisher, 10, nil)

	for range 3 {
		if err := hook.Fire(logrus.NewEntry(logrus.New())); err != nil {
			t.Fatalf("Fire returned error: %v", err)
		}
	}

	hook.Close()

	if publisher.count() != 3 {
		t.Fatalf("published = %d, want 3", publisher.count())
	}
}

func TestRabbitMQHookRecordsPipelineMetrics(t *testing.T) {
	publisher := &fakeLogPublisher{}
	metrics := &fakeLogMetrics{}
	hook := newRabbitMQHook(publisher, 10, metrics)

	if err := hook.Fire(logrus.NewEntry(logrus.New())); err != nil {
		t.Fatalf("Fire returned error: %v", err)
	}
	hook.Close()

	if got := metrics.enqueued.Load(); got != 1 {
		t.Fatalf("enqueued = %d, want 1", got)
	}
	if got := metrics.published.Load(); got != 1 {
		t.Fatalf("published = %d, want 1", got)
	}
	if got := metrics.dropped.Load(); got != 0 {
		t.Fatalf("dropped = %d, want 0", got)
	}
	if got := metrics.errors.Load(); got != 0 {
		t.Fatalf("errors = %d, want 0", got)
	}
}

func TestRabbitMQHookIgnoresFireAfterClose(t *testing.T) {
	publisher := &fakeLogPublisher{}
	hook := newRabbitMQHook(publisher, 10, nil)
	hook.Close()

	if err := hook.Fire(logrus.NewEntry(logrus.New())); err != nil {
		t.Fatalf("Fire returned error: %v", err)
	}
	if publisher.count() != 0 {
		t.Fatalf("published = %d, want 0", publisher.count())
	}
}
