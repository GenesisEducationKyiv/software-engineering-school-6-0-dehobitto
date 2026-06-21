package outbox

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/golang/mock/gomock"

	"subber/pkg/logger"
)

type MockPublisher struct {
	ctrl     *gomock.Controller
	recorder *MockPublisherMockRecorder
}

type MockPublisherMockRecorder struct {
	mock *MockPublisher
}

func NewMockPublisher(ctrl *gomock.Controller) *MockPublisher {
	mock := &MockPublisher{ctrl: ctrl}
	mock.recorder = &MockPublisherMockRecorder{mock}
	return mock
}

func (m *MockPublisher) EXPECT() *MockPublisherMockRecorder {
	return m.recorder
}

func (m *MockPublisher) Publish(ctx context.Context, topic, key string, value []byte) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Publish", ctx, topic, key, value)
	ret0, _ := ret[0].(error)
	return ret0
}

func (mr *MockPublisherMockRecorder) Publish(ctx, topic, key, value interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Publish", reflect.TypeOf((*MockPublisher)(nil).Publish), ctx, topic, key, value)
}

type MockStore struct {
	ctrl     *gomock.Controller
	recorder *MockStoreMockRecorder
}

type MockStoreMockRecorder struct {
	mock *MockStore
}

func NewMockStore(ctrl *gomock.Controller) *MockStore {
	mock := &MockStore{ctrl: ctrl}
	mock.recorder = &MockStoreMockRecorder{mock}
	return mock
}

func (m *MockStore) EXPECT() *MockStoreMockRecorder {
	return m.recorder
}

func (m *MockStore) FetchUnpublished(ctx context.Context, limit int) ([]Event, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "FetchUnpublished", ctx, limit)
	ret0, _ := ret[0].([]Event)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

func (mr *MockStoreMockRecorder) FetchUnpublished(ctx, limit interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "FetchUnpublished", reflect.TypeOf((*MockStore)(nil).FetchUnpublished), ctx, limit)
}

func (m *MockStore) MarkFailed(ctx context.Context, eventID string, cause error) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "MarkFailed", ctx, eventID, cause)
	ret0, _ := ret[0].(error)
	return ret0
}

func (mr *MockStoreMockRecorder) MarkFailed(ctx, eventID, cause interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "MarkFailed", reflect.TypeOf((*MockStore)(nil).MarkFailed), ctx, eventID, cause)
}

func (m *MockStore) MarkPublished(ctx context.Context, eventID string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "MarkPublished", ctx, eventID)
	ret0, _ := ret[0].(error)
	return ret0
}

func (mr *MockStoreMockRecorder) MarkPublished(ctx, eventID interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "MarkPublished", reflect.TypeOf((*MockStore)(nil).MarkPublished), ctx, eventID)
}

type MockLogger struct {
	ctrl     *gomock.Controller
	recorder *MockLoggerMockRecorder
}

type MockLoggerMockRecorder struct {
	mock *MockLogger
}

func NewMockLogger(ctrl *gomock.Controller) *MockLogger {
	mock := &MockLogger{ctrl: ctrl}
	mock.recorder = &MockLoggerMockRecorder{mock}
	return mock
}

func (m *MockLogger) EXPECT() *MockLoggerMockRecorder {
	return m.recorder
}

func (m *MockLogger) WithField(key string, value any) logger.Logger {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "WithField", key, value)
	ret0, _ := ret[0].(logger.Logger)
	return ret0
}

func (mr *MockLoggerMockRecorder) WithField(key, value interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "WithField", reflect.TypeOf((*MockLogger)(nil).WithField), key, value)
}

func (m *MockLogger) WithError(err error) logger.Logger {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "WithError", err)
	ret0, _ := ret[0].(logger.Logger)
	return ret0
}

func (mr *MockLoggerMockRecorder) WithError(err interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "WithError", reflect.TypeOf((*MockLogger)(nil).WithError), err)
}

func (m *MockLogger) Info(msg string) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "Info", msg)
}

func (mr *MockLoggerMockRecorder) Info(msg interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Info", reflect.TypeOf((*MockLogger)(nil).Info), msg)
}

func (m *MockLogger) Warn(msg string) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "Warn", msg)
}

func (mr *MockLoggerMockRecorder) Warn(msg interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Warn", reflect.TypeOf((*MockLogger)(nil).Warn), msg)
}

func (m *MockLogger) Error(msg string) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "Error", msg)
}

func (mr *MockLoggerMockRecorder) Error(msg interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Error", reflect.TypeOf((*MockLogger)(nil).Error), msg)
}

func (m *MockLogger) Fatal(msg string) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "Fatal", msg)
}

func (mr *MockLoggerMockRecorder) Fatal(msg interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Fatal", reflect.TypeOf((*MockLogger)(nil).Fatal), msg)
}

func TestRelay_PublishOncePublishesAndMarksEvents(t *testing.T) {
	ctrl := gomock.NewController(t)
	store := NewMockStore(ctrl)
	publisher := NewMockPublisher(ctrl)
	events := []Event{
		{EventID: "1", Topic: "topic-a", KafkaKey: "key-a", Payload: []byte(`{"a":1}`)},
		{EventID: "2", Topic: "topic-b", KafkaKey: "key-b", Payload: []byte(`{"b":2}`)},
	}
	gomock.InOrder(
		store.EXPECT().FetchUnpublished(gomock.Any(), 10).Return(events, nil),
		publisher.EXPECT().Publish(gomock.Any(), "topic-a", "key-a", []byte(`{"a":1}`)).Return(nil),
		store.EXPECT().MarkPublished(gomock.Any(), "1").Return(nil),
		publisher.EXPECT().Publish(gomock.Any(), "topic-b", "key-b", []byte(`{"b":2}`)).Return(nil),
		store.EXPECT().MarkPublished(gomock.Any(), "2").Return(nil),
	)
	relay := NewRelay(store, publisher, 10, time.Second)

	if err := relay.PublishOnce(context.Background()); err != nil {
		t.Fatalf("PublishOnce() error = %v", err)
	}
}

func TestRelay_PublishFailureMarksFailedAndStopsBatch(t *testing.T) {
	ctrl := gomock.NewController(t)
	store := NewMockStore(ctrl)
	publisher := NewMockPublisher(ctrl)
	log := NewMockLogger(ctrl)
	publishErr := errors.New("kafka down")
	events := []Event{
		{EventID: "1", Topic: "topic-a", KafkaKey: "key-a", Payload: []byte(`{"a":1}`)},
	}
	gomock.InOrder(
		store.EXPECT().FetchUnpublished(gomock.Any(), 10).Return(events, nil),
		publisher.EXPECT().Publish(gomock.Any(), "topic-a", "key-a", []byte(`{"a":1}`)).Return(publishErr),
		log.EXPECT().WithField("event_id", "1").Return(log),
		log.EXPECT().WithField("topic", "topic-a").Return(log),
		log.EXPECT().WithError(publishErr).Return(log),
		log.EXPECT().Warn("publish outbox event failed"),
		store.EXPECT().MarkFailed(gomock.Any(), "1", publishErr).Return(nil),
	)
	relay := NewRelayWithLogger(store, publisher, log, 10, time.Second)

	if err := relay.PublishOnce(context.Background()); err != nil {
		t.Fatalf("PublishOnce() error = %v", err)
	}
}

func TestRelay_ReturnsStoreErrors(t *testing.T) {
	ctrl := gomock.NewController(t)
	store := NewMockStore(ctrl)
	publisher := NewMockPublisher(ctrl)
	fetchErr := errors.New("db fetch down")
	store.EXPECT().FetchUnpublished(gomock.Any(), 10).Return(nil, fetchErr)
	if err := NewRelay(store, publisher, 10, time.Second).PublishOnce(context.Background()); !errors.Is(err, fetchErr) {
		t.Fatalf("fetch error = %v, want %v", err, fetchErr)
	}

	store = NewMockStore(ctrl)
	publisher = NewMockPublisher(ctrl)
	markPublishedErr := errors.New("mark published down")
	event := Event{EventID: "1", Topic: "topic", KafkaKey: "key", Payload: []byte(`{}`)}
	gomock.InOrder(
		store.EXPECT().FetchUnpublished(gomock.Any(), 10).Return([]Event{event}, nil),
		publisher.EXPECT().Publish(gomock.Any(), "topic", "key", []byte(`{}`)).Return(nil),
		store.EXPECT().MarkPublished(gomock.Any(), "1").Return(markPublishedErr),
	)
	err := NewRelay(store, publisher, 10, time.Second).PublishOnce(context.Background())
	if !errors.Is(err, markPublishedErr) {
		t.Fatalf("mark published error = %v, want %v", err, markPublishedErr)
	}

	store = NewMockStore(ctrl)
	publisher = NewMockPublisher(ctrl)
	markFailedErr := errors.New("mark failed down")
	publishErr := errors.New("kafka down")
	gomock.InOrder(
		store.EXPECT().FetchUnpublished(gomock.Any(), 10).Return([]Event{event}, nil),
		publisher.EXPECT().Publish(gomock.Any(), "topic", "key", []byte(`{}`)).Return(publishErr),
		store.EXPECT().MarkFailed(gomock.Any(), "1", publishErr).Return(markFailedErr),
	)
	err = NewRelay(store, publisher, 10, time.Second).PublishOnce(context.Background())
	if !errors.Is(err, markFailedErr) {
		t.Fatalf("mark failed error = %v, want %v", err, markFailedErr)
	}
}

func TestRelay_DefaultsBatchSizeAndInterval(t *testing.T) {
	ctrl := gomock.NewController(t)
	relay := NewRelay(NewMockStore(ctrl), NewMockPublisher(ctrl), 0, 0)
	if relay.batchSize != 100 {
		t.Fatalf("batchSize = %d, want 100", relay.batchSize)
	}
	if relay.interval != time.Second {
		t.Fatalf("interval = %s, want 1s", relay.interval)
	}
}
