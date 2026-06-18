package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"

	"subber/internal/config"
	gh "subber/internal/github"
	"subber/internal/infra/cache"
	"subber/internal/infra/database"
	applogger "subber/internal/logger"
	"subber/internal/metrics"
	"subber/internal/models"
	"subber/internal/routes"
	"subber/internal/service"
	"subber/internal/workers"
)

func main() {
	if err := run(); err != nil {
		logrus.WithError(err).Fatal("app error")
	}
}

func run() error {
	cfg := config.LoadConfig()

	if cfg.BaseURL == "" {
		return fmt.Errorf("BASE_URL environment variable is required")
	}

	metricRegistry := prometheus.NewRegistry()
	metrics.RegisterRuntimeCollectors(metricRegistry)
	appMetrics := metrics.New(metricRegistry)

	cleanupLogger, err := setupLogger(cfg, appMetrics)
	if err != nil {
		return fmt.Errorf("logger setup failed: %w", err)
	}
	defer cleanupLogger()
	log := applogger.New()

	connectionPool, err := database.Connect(cfg)
	if err != nil {
		return fmt.Errorf("connection to database failed: %w", err)
	}
	defer connectionPool.Close()

	if err = database.Migrate(connectionPool); err != nil {
		return fmt.Errorf("migration failed: %w", err)
	}

	repo := database.NewRepository(connectionPool)
	redisCache := cache.NewRedisCache(cfg.RedisAddr)
	if err := redisCache.Ping(context.Background()); err != nil {
		return fmt.Errorf("connection to redis failed: %w", err)
	}

	jobsChannel := make(chan models.NotificationJob, 100)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	group, groupCtx := errgroup.WithContext(ctx)

	smtpSender := workers.NewSMTPSender(cfg)
	notifier := workers.NewNotifierWorker(smtpSender, log.WithField("component", "notifier"), appMetrics)
	group.Go(withRecover(func() error {
		return notifier.Start(groupCtx, jobsChannel)
	}))

	githubClient := gh.NewClientWithBaseURL(cfg.GitHubBaseURL, cfg.GitHubToken, redisCache, log.WithField("component", "github"))

	scanner := workers.NewScannerWorker(repo, jobsChannel, githubClient, log.WithField("component", "scanner"), appMetrics)
	group.Go(withRecover(func() error {
		return scanner.StartScanner(groupCtx)
	}))

	svc := service.NewSubscriptionService(repo, cfg.BaseURL, jobsChannel, githubClient, service.RealUUIDGenerator, log.WithField("component", "service"))

	router := routes.SetupRouter(repo, svc, cfg, log, appMetrics, metricRegistry)
	srv := &http.Server{
		Addr:              ":" + cfg.ServerPort,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	group.Go(func() error {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("server error: %w", err)
		}
		return nil
	})

	group.Go(func() error {
		<-groupCtx.Done()

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err := srv.Shutdown(shutdownCtx)
		close(jobsChannel)

		return err
	})

	return group.Wait()
}

// setupLogger configures the global logrus instance and optionally adds the RabbitMQ hook.
// The returned func must be deferred to close the AMQP connection and log file on shutdown.
func setupLogger(cfg *config.Config, logMetrics applogger.LogPipelineMetrics) (func(), error) {
	logrus.SetFormatter(&logrus.JSONFormatter{})

	level, err := logrus.ParseLevel(cfg.LogLevel)
	if err != nil {
		return func() {}, fmt.Errorf("invalid LOG_LEVEL %q: %w", cfg.LogLevel, err)
	}
	logrus.SetLevel(level)

	var logFile *os.File
	if cfg.LogFile != "" {
		f, err := os.OpenFile(cfg.LogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
		if err != nil {
			return func() {}, fmt.Errorf("open log file: %w", err)
		}
		logrus.SetOutput(io.MultiWriter(os.Stdout, f))
		logFile = f
	}

	var amqpCleanup func()
	if cfg.RabbitMQURL != "" {
		hook, cleanup, err := applogger.NewRabbitMQHook(cfg.RabbitMQURL, logMetrics)
		if err != nil {
			if logFile != nil {
				_ = logFile.Close()
			}
			return func() {}, fmt.Errorf("rabbitmq hook: %w", err)
		}
		logrus.AddHook(hook)
		amqpCleanup = cleanup
	}

	return func() {
		if amqpCleanup != nil {
			amqpCleanup()
		}
		if logFile != nil {
			if err := logFile.Close(); err != nil {
				fmt.Fprintf(os.Stderr, "close log file: %v\n", err)
			}
		}
	}, nil
}

// withRecover wraps a function so that a panic is caught and returned as an error.
func withRecover(fn func() error) func() error {
	return func() (retErr error) {
		defer func() {
			if r := recover(); r != nil {
				retErr = fmt.Errorf("panic: %v", r)
			}
		}()
		return fn()
	}
}
