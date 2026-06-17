package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/sync/errgroup"

	"subber/pkg/contracts"
	"subber/pkg/kafka"
	"subber/pkg/logger"
	"subber/pkg/metrics"
	"subber/pkg/outbox"
	"subber/pkg/postgres"
	"subber/services/scanner-service/internal/cache"
	"subber/services/scanner-service/internal/config"
	scannergithub "subber/services/scanner-service/internal/github"
	"subber/services/scanner-service/internal/scanner"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "scanner-service error: %v\n", err)
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
	log := logger.New().WithField("service", "scanner-service")

	pool, err := postgres.Connect(context.Background(), cfg.Postgres())
	if err != nil {
		return fmt.Errorf("connect scanner database: %w", err)
	}
	defer pool.Close()

	if err := scanner.Migrate(context.Background(), pool); err != nil {
		return err
	}
	if err := outbox.Migrate(context.Background(), pool); err != nil {
		return err
	}
	prometheus.MustRegister(
		scanner.ReleaseDetectedTotal,
		outbox.NewBacklogGauge(pool, "scanner-service"),
	)

	redisCache := cache.NewRedisCache(cfg.RedisAddr)
	httpReleases := scannergithub.NewHTTPReleaseProvider(cfg.GitHubBaseURL, cfg.GitHubToken)
	releases := scannergithub.NewCachedReleaseProvider(redisCache, httpReleases, 45*time.Second, log.WithField("component", "github-cache"))
	service := scanner.NewService(scanner.NewRepository(pool), releases, log.WithField("component", "scanner"), cfg.ScannerBatchSize, cfg.ScannerInterval)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	group, ctx := errgroup.WithContext(ctx)
	group.Go(func() error {
		return metrics.Serve(ctx, ":"+cfg.MetricsPort)
	})
	group.Go(func() error {
		return service.Start(ctx)
	})

	watchlistConsumer := kafka.NewConsumer(cfg.KafkaBrokers, contracts.TopicWatchlistEvents, "scanner-service")
	defer watchlistConsumer.Close() //nolint:errcheck
	prometheus.MustRegister(kafka.NewConsumerLagGauge("scanner-service", contracts.TopicWatchlistEvents, watchlistConsumer))
	group.Go(func() error {
		return watchlistConsumer.Start(ctx, func(ctx context.Context, _ string, value []byte) error {
			return service.HandleWatchlistEvent(ctx, value)
		})
	})

	return group.Wait()
}
