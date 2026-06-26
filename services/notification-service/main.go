package main

import (
	"context"
	"encoding/json"
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

	notificationTransport := strings.ToLower(cfg.NotificationTransport)
	if notificationTransport == "" {
		notificationTransport = "kafka"
	}
	if notificationTransport != "kafka" && notificationTransport != "grpc" {
		return fmt.Errorf("unsupported notification transport %q", cfg.NotificationTransport)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	group, ctx := errgroup.WithContext(ctx)
	group.Go(func() error {
		return metrics.Serve(ctx, ":"+cfg.MetricsPort)
	})
	if cfg.ServerPort != "" {
		notificationServer := &http.Server{
			Addr:              ":" + cfg.ServerPort,
			Handler:           delivery.NewHTTPHandler(service, log.WithField("component", "http")),
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
	if notificationTransport == "grpc" {
		grpcListener, err := net.Listen("tcp", ":"+cfg.GRPCPort)
		if err != nil {
			return fmt.Errorf("listen grpc: %w", err)
		}
		defer grpcListener.Close() //nolint:errcheck
		grpcServer := grpc.NewServer()
		notificationv1.RegisterNotificationServiceServer(grpcServer, grpcapi.NewServer(service))
		group.Go(func() error {
			<-ctx.Done()
			grpcServer.GracefulStop()
			return nil
		})
		group.Go(func() error {
			log.WithField("addr", grpcListener.Addr().String()).Info("grpc server listening")
			if err := grpcServer.Serve(grpcListener); err != nil {
				return fmt.Errorf("serve grpc: %w", err)
			}
			return nil
		})
	}
	group.Go(func() error {
		return retryScheduler.Start(ctx)
	})
	if notificationTransport == "kafka" {
		for _, topic := range []string{
			contracts.TopicNotificationCommands,
		} {
			topic := topic
			consumer := kafka.NewConsumerWithLogger(cfg.KafkaBrokers, topic, "notification-service", log.WithField("component", "kafka-consumer").WithField("topic", topic)).
				WithDeadLetter(contracts.TopicNotificationDLQ, producer)
			defer consumer.Close() //nolint:errcheck
			prometheus.MustRegister(kafka.NewConsumerLagGauge("notification-service", topic, consumer))
			group.Go(func() error {
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

	return group.Wait()
}
