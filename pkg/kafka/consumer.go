package kafka

import (
	"context"
	"fmt"

	"github.com/segmentio/kafka-go"
)

type Handler func(ctx context.Context, key string, value []byte) error

type Consumer struct {
	reader *kafka.Reader
}

func NewConsumer(brokers []string, topic, groupID string) *Consumer {
	return &Consumer{
		reader: kafka.NewReader(kafka.ReaderConfig{
			Brokers:  brokers,
			Topic:    topic,
			GroupID:  groupID,
			MinBytes: 1,
			MaxBytes: 10e6,
		}),
	}
}

func (c *Consumer) Start(ctx context.Context, handler Handler) error {
	for {
		message, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("fetch kafka message: %w", err)
		}

		if err := handler(ctx, string(message.Key), message.Value); err != nil {
			return err
		}
		if err := c.reader.CommitMessages(ctx, message); err != nil {
			return fmt.Errorf("commit kafka message: %w", err)
		}
	}
}

func (c *Consumer) Close() error {
	return c.reader.Close()
}
