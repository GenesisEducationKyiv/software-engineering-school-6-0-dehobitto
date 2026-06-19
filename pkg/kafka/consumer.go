package kafka

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"

	"subber/pkg/contracts"
	"subber/pkg/logger"
)

type Handler func(ctx context.Context, key string, value []byte) error

type DeadLetterPublisher interface {
	Publish(ctx context.Context, topic, key string, value []byte) error
}

type messageReader interface {
	FetchMessage(ctx context.Context) (kafka.Message, error)
	CommitMessages(ctx context.Context, msgs ...kafka.Message) error
	Close() error
}

type Consumer struct {
	reader              messageReader
	log                 logger.Logger
	topic               string
	groupID             string
	deadLetterTopic     string
	deadLetterPublisher DeadLetterPublisher
}

func NewConsumerWithLogger(brokers []string, topic, groupID string, log logger.Logger) *Consumer {
	if log == nil {
		log = logger.NewNoop()
	}
	return newConsumer(kafka.NewReader(kafka.ReaderConfig{
		Brokers:  brokers,
		Topic:    topic,
		GroupID:  groupID,
		MinBytes: 1,
		MaxBytes: 10e6,
	}), topic, groupID, log)
}

func newConsumer(reader messageReader, topic, groupID string, log logger.Logger) *Consumer {
	if log == nil {
		log = logger.NewNoop()
	}
	return &Consumer{
		reader:  reader,
		log:     log,
		topic:   topic,
		groupID: groupID,
	}
}

func (c *Consumer) WithDeadLetter(topic string, publisher DeadLetterPublisher) *Consumer {
	c.deadLetterTopic = topic
	c.deadLetterPublisher = publisher
	return c
}

func (c *Consumer) Start(ctx context.Context, handler Handler) error {
	for {
		message, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			observeFailed(c.groupID, c.topic, "fetch")
			return fmt.Errorf("fetch kafka message: %w", err)
		}
		topic := c.messageTopic(message)

		if handlerErr := handler(ctx, string(message.Key), message.Value); handlerErr != nil {
			if ctx.Err() != nil {
				return nil
			}
			if errors.Is(handlerErr, ErrNonRetryable) {
				c.log.
					WithField("topic", topic).
					WithField("partition", message.Partition).
					WithField("offset", message.Offset).
					WithError(handlerErr).
					Warn("skipping non-retryable kafka message")
				if err := c.publishDeadLetter(ctx, message, handlerErr); err != nil {
					observeFailed(c.groupID, topic, "dlq")
					return fmt.Errorf("publish kafka dead letter: %w", err)
				}
				if err := c.reader.CommitMessages(ctx, message); err != nil {
					observeFailed(c.groupID, topic, "commit")
					return fmt.Errorf("commit skipped kafka message: %w", err)
				}
				observeSkipped(c.groupID, topic)
				continue
			}
			observeFailed(c.groupID, topic, "handler")
			return handlerErr
		}
		if err := c.reader.CommitMessages(ctx, message); err != nil {
			observeFailed(c.groupID, topic, "commit")
			return fmt.Errorf("commit kafka message: %w", err)
		}
		observeProcessed(c.groupID, topic)
	}
}

func (c *Consumer) Close() error {
	return c.reader.Close()
}

func (c *Consumer) Lag() int64 {
	if c == nil || c.reader == nil {
		return 0
	}
	reader, ok := c.reader.(interface{ Stats() kafka.ReaderStats })
	if !ok {
		return 0
	}
	return reader.Stats().Lag
}

func (c *Consumer) messageTopic(message kafka.Message) string {
	if message.Topic != "" {
		return message.Topic
	}
	return c.topic
}

func (c *Consumer) publishDeadLetter(ctx context.Context, message kafka.Message, cause error) error {
	if c.deadLetterPublisher == nil || c.deadLetterTopic == "" {
		return nil
	}

	now := time.Now().UTC()
	eventID := uuid.NewString()
	originalTopic := c.messageTopic(message)
	event := contracts.Envelope[contracts.DeadLetterPayload]{
		EventID:       eventID,
		EventType:     contracts.EventConsumerDeadLettered,
		OccurredAt:    now,
		Source:        c.groupID,
		CorrelationID: eventID,
		Payload: contracts.DeadLetterPayload{
			OriginalTopic:     originalTopic,
			OriginalPartition: message.Partition,
			OriginalOffset:    message.Offset,
			OriginalKey:       string(message.Key),
			OriginalValue:     message.Value,
			ConsumerGroup:     c.groupID,
			Cause:             cause.Error(),
			FailedAt:          now,
		},
	}
	value, err := json.Marshal(event)
	if err != nil {
		return err
	}
	return c.deadLetterPublisher.Publish(ctx, c.deadLetterTopic, string(message.Key), value)
}
