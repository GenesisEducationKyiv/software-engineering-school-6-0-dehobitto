package kafka

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/segmentio/kafka-go"

	"subber/pkg/contracts"
)

type MockDeadLetterPublisher struct {
	ctrl     *gomock.Controller
	recorder *MockDeadLetterPublisherMockRecorder
}

type MockDeadLetterPublisherMockRecorder struct {
	mock *MockDeadLetterPublisher
}

func NewMockDeadLetterPublisher(ctrl *gomock.Controller) *MockDeadLetterPublisher {
	mock := &MockDeadLetterPublisher{ctrl: ctrl}
	mock.recorder = &MockDeadLetterPublisherMockRecorder{mock}
	return mock
}

func (m *MockDeadLetterPublisher) EXPECT() *MockDeadLetterPublisherMockRecorder {
	return m.recorder
}

func (m *MockDeadLetterPublisher) Publish(ctx context.Context, topic, key string, value []byte) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Publish", ctx, topic, key, value)
	ret0, _ := ret[0].(error)
	return ret0
}

func (mr *MockDeadLetterPublisherMockRecorder) Publish(ctx, topic, key, value interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Publish", reflect.TypeOf((*MockDeadLetterPublisher)(nil).Publish), ctx, topic, key, value)
}

type MockmessageReader struct {
	ctrl     *gomock.Controller
	recorder *MockmessageReaderMockRecorder
}

type MockmessageReaderMockRecorder struct {
	mock *MockmessageReader
}

func NewMockmessageReader(ctrl *gomock.Controller) *MockmessageReader {
	mock := &MockmessageReader{ctrl: ctrl}
	mock.recorder = &MockmessageReaderMockRecorder{mock}
	return mock
}

func (m *MockmessageReader) EXPECT() *MockmessageReaderMockRecorder {
	return m.recorder
}

func (m *MockmessageReader) Close() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Close")
	ret0, _ := ret[0].(error)
	return ret0
}

func (mr *MockmessageReaderMockRecorder) Close() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Close", reflect.TypeOf((*MockmessageReader)(nil).Close))
}

func (m *MockmessageReader) CommitMessages(ctx context.Context, msgs ...kafka.Message) error {
	m.ctrl.T.Helper()
	varargs := []interface{}{ctx}
	for _, a := range msgs {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "CommitMessages", varargs...)
	ret0, _ := ret[0].(error)
	return ret0
}

func (mr *MockmessageReaderMockRecorder) CommitMessages(ctx interface{}, msgs ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{ctx}, msgs...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CommitMessages", reflect.TypeOf((*MockmessageReader)(nil).CommitMessages), varargs...)
}

func (m *MockmessageReader) FetchMessage(ctx context.Context) (kafka.Message, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "FetchMessage", ctx)
	ret0, _ := ret[0].(kafka.Message)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

func (mr *MockmessageReaderMockRecorder) FetchMessage(ctx interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "FetchMessage", reflect.TypeOf((*MockmessageReader)(nil).FetchMessage), ctx)
}

func TestConsumer_StartCommitsSuccessfulMessages(t *testing.T) {
	ctrl := gomock.NewController(t)
	reader := NewMockmessageReader(ctrl)
	ctx, cancel := context.WithCancel(context.Background())
	first := kafka.Message{Topic: "topic", Offset: 1, Key: []byte("a"), Value: []byte("first")}
	second := kafka.Message{Topic: "topic", Offset: 2, Key: []byte("b"), Value: []byte("second")}
	gomock.InOrder(
		reader.EXPECT().FetchMessage(ctx).Return(first, nil),
		reader.EXPECT().CommitMessages(ctx, first).Return(nil),
		reader.EXPECT().FetchMessage(ctx).Return(second, nil),
		reader.EXPECT().CommitMessages(ctx, second).Return(nil),
		reader.EXPECT().FetchMessage(ctx).Return(kafka.Message{}, context.Canceled),
	)
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
}

func TestConsumer_StartSkipsAndCommitsNonRetryableErrors(t *testing.T) {
	ctrl := gomock.NewController(t)
	reader := NewMockmessageReader(ctrl)
	ctx, cancel := context.WithCancel(context.Background())
	bad := kafka.Message{Topic: "topic", Offset: 1, Key: []byte("bad"), Value: []byte("not-json")}
	good := kafka.Message{Topic: "topic", Offset: 2, Key: []byte("good"), Value: []byte(`{"ok":true}`)}
	gomock.InOrder(
		reader.EXPECT().FetchMessage(ctx).Return(bad, nil),
		reader.EXPECT().CommitMessages(ctx, bad).Return(nil),
		reader.EXPECT().FetchMessage(ctx).Return(good, nil),
		reader.EXPECT().CommitMessages(ctx, good).Return(nil),
		reader.EXPECT().FetchMessage(ctx).Return(kafka.Message{}, context.Canceled),
	)
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
}

func TestConsumer_StartStopsWithoutCommitOnRetryableErrors(t *testing.T) {
	ctrl := gomock.NewController(t)
	reader := NewMockmessageReader(ctrl)
	message := kafka.Message{Topic: "topic", Offset: 1, Key: []byte("retry"), Value: []byte(`{"ok":true}`)}
	reader.EXPECT().FetchMessage(gomock.Any()).Return(message, nil)
	consumer := newConsumer(reader, "topic", "group", nil)
	retryErr := errors.New("database down")

	err := consumer.Start(context.Background(), func(context.Context, string, []byte) error {
		return Retryable(retryErr)
	})

	if !errors.Is(err, retryErr) {
		t.Fatalf("Start() error = %v, want %v", err, retryErr)
	}
}

func TestConsumer_StartStopsWithoutCommitOnPlainErrors(t *testing.T) {
	ctrl := gomock.NewController(t)
	reader := NewMockmessageReader(ctrl)
	message := kafka.Message{Topic: "topic", Offset: 1, Key: []byte("retry"), Value: []byte(`{"ok":true}`)}
	reader.EXPECT().FetchMessage(gomock.Any()).Return(message, nil)
	consumer := newConsumer(reader, "topic", "group", nil)
	retryErr := errors.New("database down")

	err := consumer.Start(context.Background(), func(context.Context, string, []byte) error {
		return retryErr
	})

	if !errors.Is(err, retryErr) {
		t.Fatalf("Start() error = %v, want %v", err, retryErr)
	}
}

func TestConsumer_StartReturnsCommitErrorForSkippedMessage(t *testing.T) {
	ctrl := gomock.NewController(t)
	reader := NewMockmessageReader(ctrl)
	commitErr := errors.New("commit failed")
	message := kafka.Message{Topic: "topic", Offset: 1, Value: []byte("bad")}
	gomock.InOrder(
		reader.EXPECT().FetchMessage(gomock.Any()).Return(message, nil),
		reader.EXPECT().CommitMessages(gomock.Any(), message).Return(commitErr),
	)
	consumer := newConsumer(reader, "topic", "group", nil)

	err := consumer.Start(context.Background(), func(context.Context, string, []byte) error {
		return NonRetryable(errors.New("malformed message"))
	})

	if !errors.Is(err, commitErr) {
		t.Fatalf("Start() error = %v, want %v", err, commitErr)
	}
}

func TestConsumer_StartPublishesNonRetryableErrorsToDLQBeforeCommit(t *testing.T) {
	ctrl := gomock.NewController(t)
	reader := NewMockmessageReader(ctrl)
	publisher := NewMockDeadLetterPublisher(ctrl)
	ctx, cancel := context.WithCancel(context.Background())
	bad := kafka.Message{Topic: "source-topic", Partition: 2, Offset: 42, Key: []byte("bad-key"), Value: []byte("not-json")}
	good := kafka.Message{Topic: "source-topic", Partition: 2, Offset: 43, Key: []byte("good-key"), Value: []byte(`{"ok":true}`)}
	var dlqTopic string
	var dlqKey string
	var dlqValue []byte
	gomock.InOrder(
		reader.EXPECT().FetchMessage(ctx).Return(bad, nil),
		publisher.EXPECT().
			Publish(ctx, "source-topic.dlq", "bad-key", gomock.Any()).
			DoAndReturn(func(_ context.Context, topic, key string, value []byte) error {
				dlqTopic = topic
				dlqKey = key
				dlqValue = value
				return nil
			}),
		reader.EXPECT().CommitMessages(ctx, bad).Return(nil),
		reader.EXPECT().FetchMessage(ctx).Return(good, nil),
		reader.EXPECT().CommitMessages(ctx, good).Return(nil),
		reader.EXPECT().FetchMessage(ctx).Return(kafka.Message{}, context.Canceled),
	)
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
	if dlqTopic != "source-topic.dlq" || dlqKey != "bad-key" {
		t.Fatalf("published DLQ routing = (%q, %q)", dlqTopic, dlqKey)
	}

	var event contracts.Envelope[contracts.DeadLetterPayload]
	if err := json.Unmarshal(dlqValue, &event); err != nil {
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
	ctrl := gomock.NewController(t)
	reader := NewMockmessageReader(ctrl)
	publisher := NewMockDeadLetterPublisher(ctrl)
	publishErr := errors.New("dlq publish failed")
	message := kafka.Message{Topic: "source-topic", Offset: 42, Key: []byte("bad-key"), Value: []byte("not-json")}
	gomock.InOrder(
		reader.EXPECT().FetchMessage(gomock.Any()).Return(message, nil),
		publisher.EXPECT().Publish(gomock.Any(), "source-topic.dlq", "bad-key", gomock.Any()).Return(publishErr),
	)
	consumer := newConsumer(reader, "source-topic", "consumer-group", nil).
		WithDeadLetter("source-topic.dlq", publisher)

	err := consumer.Start(context.Background(), func(context.Context, string, []byte) error {
		return NonRetryable(errors.New("malformed message"))
	})

	if !errors.Is(err, publishErr) {
		t.Fatalf("Start() error = %v, want %v", err, publishErr)
	}
}
