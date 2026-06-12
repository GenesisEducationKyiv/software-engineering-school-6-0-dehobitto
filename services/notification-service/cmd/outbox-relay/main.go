package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"subber/pkg/kafka"
	"subber/pkg/outbox"
	"subber/pkg/postgres"
	"subber/services/notification-service/internal/config"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "notification outbox relay error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg := config.Load()
	pool, err := postgres.Connect(context.Background(), cfg.Postgres())
	if err != nil {
		return fmt.Errorf("connect notification database: %w", err)
	}
	defer pool.Close()

	producer := kafka.NewProducer(cfg.KafkaBrokers)
	defer producer.Close() //nolint:errcheck

	relay := outbox.NewRelay(outbox.NewRepository(pool), producer, 100, time.Second)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return relay.Start(ctx)
}
