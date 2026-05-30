package logger

import (
	"fmt"
	"os"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/sirupsen/logrus"
)

const (
	logsQueue   = "logs"
	dlxExchange = "logs.dlx"
	deadQueue   = "logs.dead"
)

// RabbitMQHook publishes each logrus entry as a persistent JSON message.
// Unprocessable messages are routed to logs.dead via a dead-letter exchange.
type RabbitMQHook struct {
	ch *amqp.Channel
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
func NewRabbitMQHook(url string) (*RabbitMQHook, func(), error) {
	_, ch, cleanup, err := dialAMQP(url)
	if err != nil {
		return nil, nil, err
	}

	if err := declareTopology(ch); err != nil {
		cleanup()
		return nil, nil, err
	}

	return &RabbitMQHook{ch: ch}, cleanup, nil
}

func (h *RabbitMQHook) Levels() []logrus.Level { return logrus.AllLevels }

func (h *RabbitMQHook) Fire(entry *logrus.Entry) error {
	b, err := entry.Bytes()
	if err != nil {
		return err
	}
	if err = h.ch.Publish("", logsQueue, false, false, amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		Body:         b,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "rabbitmq hook: publish failed: %v — original entry: %s\n", err, b)
	}
	return nil
}
