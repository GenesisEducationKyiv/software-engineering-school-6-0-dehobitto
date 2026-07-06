package kafka

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/segmentio/kafka-go"

	"subber/pkg/contracts"
)

type fakeReader struct {
	messages       []kafka.Message
	fetchErr       error
	commitErr      error
	fetches        int
	committed      []kafka.Message
	closeCallCount int
}

type fakeDeadLetterPublisher struct {
	err      error
	messages []publishedDeadLetter
}

type publishedDeadLetter struct {
	topic string
	key   string
	value []byte
}

func (p *fakeDeadLetterPublisher) Publish(_ context.Context, topic, key string, value []byte) error {
	if p.err != nil {
		return p.err
	}
	p.messages = append(p.messages, publishedDeadLetter{topic: topic, key: key, value: value})
	return nil
}

func (r *fakeReader) FetchMessage(ctx context.Context) (kafka.Message, error) {
	if r.fetchErr != nil {
		return kafka.Message{}, r.fetchErr
	}
	if r.fetches >= len(r.messages) {
		<-ctx.Done()
		return kafka.Message{}, ctx.Err()
	}
	message := r.messages[r.fetches]
	r.fetches++
	return message, nil
}

func (r *fakeReader) CommitMessages(_ context.Context, msgs ...kafka.Message) error {
	if r.commitErr != nil {
		return r.commitErr
	}
	r.committed = append(r.committed, msgs...)
	return nil
}

func (r *fakeReader) Close() error {
	r.closeCallCount++
	return nil
}

func TestConsumer_StartCommitsSuccessfulMessages(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	reader := &fakeReader{messages: []kafka.Message{
		{Topic: "topic", Offset: 1, Key: []byte("a"), Value: []byte("first")},
		{Topic: "topic", Offset: 2, Key: []byte("b"), Value: []byte("second")},
	}}
	consumer := newConsumer(reader, "topic", "group", nil)

	handled := 0
	err := consumer.Start(ctx, func(_ context.Context, key string, _ []byte) error {
		handled++
		if handled == 1 && key != "a" {
			t.Fatalf("first key = %q, want a", key)
		}
		if handled == 2 {
			cancel()
		}
		return nil
	})

	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if handled != 2 {
		t.Fatalf("handled = %d, want 2", handled)
	}
	if len(reader.committed) != 2 || reader.committed[0].Offset != 1 || reader.committed[1].Offset != 2 {
		t.Fatalf("committed offsets = %#v, want 1 and 2", committedOffsets(reader.committed))
	}
}

func TestConsumer_StartSkipsAndCommitsNonRetryableErrors(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	reader := &fakeReader{messages: []kafka.Message{
		{Topic: "topic", Offset: 1, Key: []byte("bad"), Value: []byte("not-json")},
		{Topic: "topic", Offset: 2, Key: []byte("good"), Value: []byte(`{"ok":true}`)},
	}}
	consumer := newConsumer(reader, "topic", "group", nil)

	handled := 0
	err := consumer.Start(ctx, func(_ context.Context, key string, _ []byte) error {
		handled++
		if key == "bad" {
			return NonRetryable(errors.New("malformed message"))
		}
		cancel()
		return nil
	})

	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if handled != 2 {
		t.Fatalf("handled = %d, want 2", handled)
	}
	if len(reader.committed) != 2 || reader.committed[0].Offset != 1 || reader.committed[1].Offset != 2 {
		t.Fatalf("committed offsets = %#v, want 1 and 2", committedOffsets(reader.committed))
	}
}

func TestConsumer_StartStopsWithoutCommitOnRetryableErrors(t *testing.T) {
	reader := &fakeReader{messages: []kafka.Message{
		{Topic: "topic", Offset: 1, Key: []byte("retry"), Value: []byte(`{"ok":true}`)},
	}}
	consumer := newConsumer(reader, "topic", "group", nil)
	retryErr := errors.New("database down")

	err := consumer.Start(context.Background(), func(context.Context, string, []byte) error {
		return Retryable(retryErr)
	})

	if !errors.Is(err, retryErr) {
		t.Fatalf("Start() error = %v, want %v", err, retryErr)
	}
	if len(reader.committed) != 0 {
		t.Fatalf("committed offsets = %#v, want none", committedOffsets(reader.committed))
	}
}

func TestConsumer_StartStopsWithoutCommitOnPlainErrors(t *testing.T) {
	reader := &fakeReader{messages: []kafka.Message{
		{Topic: "topic", Offset: 1, Key: []byte("retry"), Value: []byte(`{"ok":true}`)},
	}}
	consumer := newConsumer(reader, "topic", "group", nil)
	retryErr := errors.New("database down")

	err := consumer.Start(context.Background(), func(context.Context, string, []byte) error {
		return retryErr
	})

	if !errors.Is(err, retryErr) {
		t.Fatalf("Start() error = %v, want %v", err, retryErr)
	}
	if len(reader.committed) != 0 {
		t.Fatalf("committed offsets = %#v, want none", committedOffsets(reader.committed))
	}
}

func TestConsumer_StartReturnsCommitErrorForSkippedMessage(t *testing.T) {
	commitErr := errors.New("commit failed")
	reader := &fakeReader{
		messages:  []kafka.Message{{Topic: "topic", Offset: 1, Value: []byte("bad")}},
		commitErr: commitErr,
	}
	consumer := newConsumer(reader, "topic", "group", nil)

	err := consumer.Start(context.Background(), func(context.Context, string, []byte) error {
		return NonRetryable(errors.New("malformed message"))
	})

	if !errors.Is(err, commitErr) {
		t.Fatalf("Start() error = %v, want %v", err, commitErr)
	}
}

func TestConsumer_StartPublishesNonRetryableErrorsToDLQBeforeCommit(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	reader := &fakeReader{messages: []kafka.Message{
		{Topic: "source-topic", Partition: 2, Offset: 42, Key: []byte("bad-key"), Value: []byte("not-json")},
		{Topic: "source-topic", Partition: 2, Offset: 43, Key: []byte("good-key"), Value: []byte(`{"ok":true}`)},
	}}
	publisher := &fakeDeadLetterPublisher{}
	consumer := newConsumer(reader, "source-topic", "consumer-group", nil).
		WithDeadLetter("source-topic.dlq", publisher)

	handled := 0
	err := consumer.Start(ctx, func(context.Context, string, []byte) error {
		handled++
		if handled == 2 {
			cancel()
			return nil
		}
		return NonRetryable(errors.New("malformed message"))
	})

	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if len(publisher.messages) != 1 {
		t.Fatalf("published DLQ messages = %d, want 1", len(publisher.messages))
	}
	if len(reader.committed) != 2 || reader.committed[0].Offset != 42 || reader.committed[1].Offset != 43 {
		t.Fatalf("committed offsets = %#v, want 42 and 43", committedOffsets(reader.committed))
	}
	if publisher.messages[0].topic != "source-topic.dlq" || publisher.messages[0].key != "bad-key" {
		t.Fatalf("published DLQ routing = (%q, %q)", publisher.messages[0].topic, publisher.messages[0].key)
	}

	var event contracts.Envelope[contracts.DeadLetterPayload]
	if err := json.Unmarshal(publisher.messages[0].value, &event); err != nil {
		t.Fatalf("DLQ value is not dead letter envelope: %v", err)
	}
	if event.EventType != contracts.EventConsumerDeadLettered {
		t.Fatalf("event type = %q, want %q", event.EventType, contracts.EventConsumerDeadLettered)
	}
	if event.Payload.OriginalTopic != "source-topic" ||
		event.Payload.OriginalPartition != 2 ||
		event.Payload.OriginalOffset != 42 ||
		event.Payload.OriginalKey != "bad-key" ||
		string(event.Payload.OriginalValue) != "not-json" ||
		event.Payload.ConsumerGroup != "consumer-group" {
		t.Fatalf("DLQ payload = %#v", event.Payload)
	}
}

func TestConsumer_StartDoesNotCommitWhenDLQPublishFails(t *testing.T) {
	publishErr := errors.New("dlq publish failed")
	reader := &fakeReader{messages: []kafka.Message{
		{Topic: "source-topic", Offset: 42, Key: []byte("bad-key"), Value: []byte("not-json")},
	}}
	consumer := newConsumer(reader, "source-topic", "consumer-group", nil).
		WithDeadLetter("source-topic.dlq", &fakeDeadLetterPublisher{err: publishErr})

	err := consumer.Start(context.Background(), func(context.Context, string, []byte) error {
		return NonRetryable(errors.New("malformed message"))
	})

	if !errors.Is(err, publishErr) {
		t.Fatalf("Start() error = %v, want %v", err, publishErr)
	}
	if len(reader.committed) != 0 {
		t.Fatalf("committed offsets = %#v, want none", committedOffsets(reader.committed))
	}
}

func committedOffsets(messages []kafka.Message) []int64 {
	offsets := make([]int64, 0, len(messages))
	for _, message := range messages {
		offsets = append(offsets, message.Offset)
	}
	return offsets
}
