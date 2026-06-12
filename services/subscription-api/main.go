package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/sync/errgroup"

	"subber/pkg/contracts"
	"subber/pkg/kafka"
	"subber/pkg/logger"
	"subber/pkg/postgres"
	"subber/services/subscription-api/internal/config"
	"subber/services/subscription-api/internal/httpapi"
	"subber/services/subscription-api/internal/subscription"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "subscription-api error: %v\n", err)
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

	metricRegistry := prometheus.NewRegistry()
	metricRegistry.MustRegister(
		prometheus.NewGoCollector(),
		prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}),
	)
	log := logger.New().WithField("service", "subscription-api")

	pool, err := postgres.Connect(context.Background(), cfg.Postgres())
	if err != nil {
		return fmt.Errorf("connect subscription database: %w", err)
	}
	defer pool.Close()

	if err := subscription.Migrate(context.Background(), pool); err != nil {
		return fmt.Errorf("migrate subscription database: %w", err)
	}

	repo := subscription.NewRepository(pool)
	githubClient := subscription.NewHTTPGitHubClient(cfg.GitHubBaseURL, cfg.GitHubToken)
	notificationPublisher := subscription.NewOutboxNotificationPublisher(pool, cfg.BaseURL)
	svc := subscription.NewService(
		repo,
		notificationPublisher,
		githubClient,
		log.WithField("component", "service"),
	)
	releaseExpander := subscription.NewReleaseExpander(repo, notificationPublisher)

	router := httpapi.SetupRouter(httpapi.RouterDeps{
		APIKey:   cfg.APIKey,
		Repo:     repo,
		Service:  svc,
		Logger:   log,
		Gatherer: metricRegistry,
	})
	srv := &http.Server{
		Addr:              ":" + cfg.ServerPort,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	group, ctx := errgroup.WithContext(ctx)

	group.Go(func() error {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("server error: %w", err)
		}
		return nil
	})

	group.Go(func() error {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	})

	releaseConsumer := kafka.NewConsumer(cfg.KafkaBrokers, contracts.TopicReleaseEvents, "subscription-api")
	defer releaseConsumer.Close() //nolint:errcheck
	group.Go(func() error {
		return releaseConsumer.Start(ctx, func(ctx context.Context, _ string, value []byte) error {
			var event contracts.Envelope[contracts.ReleaseDetectedPayload]
			if err := json.Unmarshal(value, &event); err != nil {
				return fmt.Errorf("decode release event: %w", err)
			}
			return releaseExpander.Expand(ctx, event)
		})
	})

	return group.Wait()
}
