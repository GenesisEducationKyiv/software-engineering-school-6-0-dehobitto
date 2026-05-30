package logger

import (
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

// NewRabbitMQHook dials RabbitMQ, declares queue topology, and returns the hook.
// The returned cleanup func closes the channel and connection — defer it in main.
func NewRabbitMQHook(url string) (*RabbitMQHook, func(), error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, nil, err
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, nil, err
	}

	cleanup := func() { ch.Close(); conn.Close() }

	if err = ch.ExchangeDeclare(dlxExchange, "direct", true, false, false, false, nil); err != nil {
		cleanup()
		return nil, nil, err
	}

	if _, err = ch.QueueDeclare(deadQueue, true, false, false, false, nil); err != nil {
		cleanup()
		return nil, nil, err
	}

	if err = ch.QueueBind(deadQueue, logsQueue, dlxExchange, false, nil); err != nil {
		cleanup()
		return nil, nil, err
	}

	args := amqp.Table{"x-dead-letter-exchange": dlxExchange}
	if _, err = ch.QueueDeclare(logsQueue, true, false, false, false, args); err != nil {
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
	return h.ch.Publish("", logsQueue, false, false, amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		Body:         b,
	})
}
