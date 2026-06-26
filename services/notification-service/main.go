package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"

	"subber/pkg/contracts"
	notificationv1 "subber/pkg/gen/notification/v1"
	"subber/pkg/kafka"
	"subber/pkg/logger"
	"subber/pkg/metrics"
	"subber/pkg/outbox"
	"subber/pkg/postgres"
	"subber/services/notification-service/internal/config"
	"subber/services/notification-service/internal/delivery"
	"subber/services/notification-service/internal/email"
	"subber/services/notification-service/internal/grpcapi"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "notification-service error: %v\n", err)
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
	log := logger.New().WithField("service", "notification-service")

	pool, err := postgres.Connect(context.Background(), cfg.Postgres())
	if err != nil {
		return fmt.Errorf("connect notification database: %w", err)
	}
	defer pool.Close()
	prometheus.MustRegister(outbox.NewCollector(pool, "notification-service"))
	repo := delivery.NewRepository(pool)

	producer := kafka.NewProducer(cfg.KafkaBrokers)
	defer producer.Close() //nolint:errcheck

	service := delivery.NewService(
		repo,
		email.NewSMTPSender(email.Config{
			SMTPHost:     cfg.SMTPHost,
			SMTPPort:     cfg.SMTPPort,
			SMTPEmail:    cfg.SMTPEmail,
			SMTPPassword: cfg.SMTPPassword,
		}),
		delivery.NewKafkaRetryPublisher(producer),
		log.WithField("component", "delivery"),
		cfg.NotificationRetryAttempts,
		cfg.NotificationRetryDelays,
	)
	retryScheduler := delivery.NewRetryScheduler(repo, log.WithField("component", "retry-scheduler"), 100, time.Second)

	notificationTransport, err := notificationTransport(cfg.NotificationTransport)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	group, ctx := errgroup.WithContext(ctx)
	group.Go(func() error {
		return metrics.Serve(ctx, ":"+cfg.MetricsPort)
	})
	registerHTTPServer(ctx, group, cfg.ServerPort, service, log.WithField("component", "http"))
	if notificationTransport == "grpc" {
		if err := registerGRPCServer(ctx, group, cfg.GRPCPort, service, log); err != nil {
			return err
		}
	}
	group.Go(func() error {
		return retryScheduler.Start(ctx)
	})
	if notificationTransport == "kafka" {
		registerNotificationCommandConsumers(ctx, group, cfg.KafkaBrokers, producer, service, log)
	}

	return group.Wait()
}

func notificationTransport(value string) (string, error) {
	transport := strings.ToLower(value)
	if transport == "" {
		transport = "kafka"
	}
	if transport != "kafka" && transport != "grpc" {
		return "", fmt.Errorf("unsupported notification transport %q", value)
	}
	return transport, nil
}

func registerHTTPServer(ctx context.Context, group *errgroup.Group, port string, service *delivery.Service, log logger.Logger) {
	if port == "" {
		return
	}
	notificationServer := &http.Server{
		Addr:              ":" + port,
		Handler:           delivery.NewHTTPHandler(service, log),
		ReadHeaderTimeout: 10 * time.Second,
	}
	group.Go(func() error {
		if err := notificationServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			return fmt.Errorf("notification http server: %w", err)
		}
		return nil
	})
	group.Go(func() error {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return notificationServer.Shutdown(shutdownCtx)
	})
}

func registerGRPCServer(ctx context.Context, group *errgroup.Group, port string, service *delivery.Service, log logger.Logger) error {
	grpcListener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		return fmt.Errorf("listen grpc: %w", err)
	}
	grpcServer := grpc.NewServer()
	notificationv1.RegisterNotificationServiceServer(grpcServer, grpcapi.NewServer(service))
	group.Go(func() error {
		<-ctx.Done()
		grpcServer.GracefulStop()
		return nil
	})
	group.Go(func() error {
		log.WithField("addr", grpcListener.Addr().String()).Info("grpc server listening")
		if err := grpcServer.Serve(grpcListener); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			return fmt.Errorf("serve grpc: %w", err)
		}
		return nil
	})
	return nil
}

func registerNotificationCommandConsumers(ctx context.Context, group *errgroup.Group, brokers []string, producer *kafka.Producer, service *delivery.Service, log logger.Logger) {
	for _, topic := range []string{contracts.TopicNotificationCommands} {
		topic := topic
		consumer := kafka.NewConsumerWithLogger(brokers, topic, "notification-service", log.WithField("component", "kafka-consumer").WithField("topic", topic)).
			WithDeadLetter(contracts.TopicNotificationDLQ, producer)
		prometheus.MustRegister(kafka.NewConsumerLagGauge("notification-service", topic, consumer))
		group.Go(func() error {
			defer consumer.Close() //nolint:errcheck
			return consumer.Start(ctx, func(ctx context.Context, _ string, value []byte) error {
				var event contracts.Envelope[contracts.NotificationSendRequestedPayload]
				if err := json.Unmarshal(value, &event); err != nil {
					return kafka.NonRetryable(fmt.Errorf("decode notification command: %w", err))
				}
				if event.EventType != contracts.EventNotificationRequested {
					return kafka.NonRetryable(fmt.Errorf("unsupported notification event type %q", event.EventType))
				}
				return service.Process(ctx, event.Payload)
			})
		})
	}
}
