package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/sync/errgroup"

	"subber/pkg/contracts"
	"subber/pkg/kafka"
	"subber/pkg/logger"
	"subber/pkg/postgres"
	"subber/services/subscription-api/internal/config"
	"subber/services/subscription-api/internal/watchsaga"
)

const retrySweepInterval = 10 * time.Second

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "subscription-api saga-orchestrator error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg := config.Load()
	cleanupLogs, err := logger.Configure(cfg.LogLevel, cfg.LogSidecarEnabled, cfg.LogSidecarURL, cfg.LogFile)
	if err != nil {
		return fmt.Errorf("configure logger: %w", err)
	}
	defer cleanupLogs()
	log := logger.New().WithField("service", "subscription-api-saga-orchestrator")

	pool, err := postgres.Connect(context.Background(), cfg.Postgres())
	if err != nil {
		return fmt.Errorf("connect subscription database: %w", err)
	}
	defer pool.Close()

	orchestrator := watchsaga.New(pool)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	group, ctx := errgroup.WithContext(ctx)

	requestConsumer := kafka.NewConsumerWithLogger(cfg.KafkaBrokers,
		contracts.TopicWatchlistSagaRequests,
		"subscription-api-saga-orchestrator",
		log.WithField("component", "kafka-consumer").WithField("topic", contracts.TopicWatchlistSagaRequests),
	)
	defer requestConsumer.Close() //nolint:errcheck
	group.Go(func() error {
		return requestConsumer.Start(
			ctx, func(ctx context.Context, _ string, value []byte) error {
				var event contracts.Envelope[contracts.RepoWatchSagaPayload]
				if err := json.Unmarshal(value, &event); err != nil {
					return kafka.NonRetryable(fmt.Errorf("decode repo watch saga request: %w", err))
				}

				return orchestrator.HandleRequest(ctx, event)
			})
	})

	ackConsumer := kafka.NewConsumerWithLogger(cfg.KafkaBrokers,
		contracts.TopicWatchlistSagaEvents,
		"subscription-api-saga-orchestrator",
		log.WithField("component", "kafka-consumer").WithField("topic", contracts.TopicWatchlistSagaEvents),
	)
	defer ackConsumer.Close() //nolint:errcheck
	group.Go(func() error {
		return ackConsumer.Start(ctx, func(ctx context.Context, _ string, value []byte) error {
			var event contracts.Envelope[contracts.RepoWatchAckPayload]
			if err := json.Unmarshal(value, &event); err != nil {
				return kafka.NonRetryable(fmt.Errorf("decode repo watch ack: %w", err))
			}
			return orchestrator.HandleAck(ctx, event)
		})
	})

	group.Go(func() error {
		ticker := time.NewTicker(retrySweepInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return nil
			case <-ticker.C:
				if err := orchestrator.RetryDue(ctx, watchsaga.DefaultRetryLimit); err != nil {
					log.WithError(err).Error("saga retry tick failed")
				}
			}
		}
	})

	return group.Wait()
}
