package logger

import (
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/sirupsen/logrus"
)

const (
	logsQueue            = "logs"
	dlxExchange          = "logs.dlx"
	deadQueue            = "logs.dead"
	defaultLogBufferSize = 1000
	shutdownDrainTimeout = 5 * time.Second
)

// RabbitMQHook asynchronously publishes each logrus entry as a persistent JSON message.
// Unprocessable messages are routed to logs.dead via a dead-letter exchange.
type RabbitMQHook struct {
	publisher logPublisher
	entries   chan []byte
	closed    bool
	mu        sync.RWMutex
	dropped   atomic.Uint64
	wg        sync.WaitGroup
	metrics   LogPipelineMetrics
}

type LogPipelineMetrics interface {
	IncLogEntriesEnqueued()
	IncLogEntriesDropped()
	IncLogEntriesPublished()
	IncLogPublishErrors()
}

type logPublisher interface {
	Publish(body []byte) error
}

type amqpLogPublisher struct {
	ch *amqp.Channel
}

func (p amqpLogPublisher) Publish(body []byte) error {
	return p.ch.Publish("", logsQueue, false, false, amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		Body:         body,
	})
}

// dialAMQP dials the broker and opens a channel. The returned cleanup closes
// both on shutdown; callers must invoke it on error paths too.
func dialAMQP(url string) (*amqp.Connection, *amqp.Channel, func(), error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, nil, nil, err
	}

	ch, err := conn.Channel()
	if err != nil {
		if cerr := conn.Close(); cerr != nil {
			fmt.Fprintf(os.Stderr, "close amqp connection: %v\n", cerr)
		}
		return nil, nil, nil, err
	}

	cleanup := func() {
		if err := ch.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "close amqp channel: %v\n", err)
		}
		if err := conn.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "close amqp connection: %v\n", err)
		}
	}

	return conn, ch, cleanup, nil
}

// declareTopology declares the DLX exchange, dead-letter queue, binding, and
// main logs queue on the provided channel.
func declareTopology(ch *amqp.Channel) error {
	if err := ch.ExchangeDeclare(dlxExchange, "direct", true, false, false, false, nil); err != nil {
		return err
	}

	if _, err := ch.QueueDeclare(deadQueue, true, false, false, false, nil); err != nil {
		return err
	}

	if err := ch.QueueBind(deadQueue, logsQueue, dlxExchange, false, nil); err != nil {
		return err
	}

	args := amqp.Table{"x-dead-letter-exchange": dlxExchange}
	if _, err := ch.QueueDeclare(logsQueue, true, false, false, false, args); err != nil {
		return err
	}

	return nil
}

// NewRabbitMQHook connects to the broker, declares the queue topology, and
// returns a ready hook plus a cleanup function to close the connection.
func NewRabbitMQHook(url string, metrics LogPipelineMetrics) (*RabbitMQHook, func(), error) {
	_, ch, cleanup, err := dialAMQP(url)
	if err != nil {
		return nil, nil, err
	}

	if err := declareTopology(ch); err != nil {
		cleanup()
		return nil, nil, err
	}

	hook := newRabbitMQHook(amqpLogPublisher{ch: ch}, defaultLogBufferSize, metrics)

	return hook, func() {
		hook.Close()
		cleanup()
	}, nil
}

func (h *RabbitMQHook) Levels() []logrus.Level { return logrus.AllLevels }

func (h *RabbitMQHook) Fire(entry *logrus.Entry) error {
	b, err := entry.Bytes()
	if err != nil {
		return err
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	if h.closed {
		return nil
	}

	select {
	case h.entries <- b:
		if h.metrics != nil {
			h.metrics.IncLogEntriesEnqueued()
		}
	default:
		dropped := h.dropped.Add(1)
		if h.metrics != nil {
			h.metrics.IncLogEntriesDropped()
		}
		fmt.Fprintf(os.Stderr, "rabbitmq hook: buffer full, dropped log entry (dropped_total=%d)\n", dropped)
	}
	return nil
}

func newRabbitMQHook(publisher logPublisher, bufferSize int, metrics LogPipelineMetrics) *RabbitMQHook {
	if bufferSize <= 0 {
		bufferSize = defaultLogBufferSize
	}

	h := &RabbitMQHook{
		publisher: publisher,
		entries:   make(chan []byte, bufferSize),
		metrics:   metrics,
	}
	h.wg.Add(1)
	go h.publishLoop()

	return h
}

func (h *RabbitMQHook) publishLoop() {
	defer h.wg.Done()

	for b := range h.entries {
		if err := h.publish(b); err != nil {
			if h.metrics != nil {
				h.metrics.IncLogPublishErrors()
			}
			fmt.Fprintf(os.Stderr, "rabbitmq hook: publish failed: %v; original entry: %s\n", err, b)
			continue
		}
		if h.metrics != nil {
			h.metrics.IncLogEntriesPublished()
		}
	}
}

func (h *RabbitMQHook) publish(body []byte) error {
	return h.publisher.Publish(body)
}

func (h *RabbitMQHook) Close() {
	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		return
	}
	h.closed = true

	close(h.entries)
	h.mu.Unlock()

	done := make(chan struct{})
	go func() {
		h.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(shutdownDrainTimeout):
		fmt.Fprintf(os.Stderr, "rabbitmq hook: shutdown drain timed out after %s\n", shutdownDrainTimeout)
	}
}
