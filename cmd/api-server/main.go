package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/sync/errgroup"

	"subber/internal/config"
	gh "subber/internal/github"
	"subber/internal/infra/cache"
	"subber/internal/infra/database"
	"subber/internal/models"
	"subber/internal/routes"
	"subber/internal/service"
	"subber/internal/workers"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("App error: %v", err)
	}
}

func run() error {
	cfg := config.LoadConfig()

	if cfg.BaseURL == "" {
		return fmt.Errorf("BASE_URL environment variable is required")
	}

	connectionPool, err := database.Connect(cfg)
	if err != nil {
		return fmt.Errorf("connection to database failed: %w", err)
	}
	defer connectionPool.Close()

	if err = database.Migrate(connectionPool); err != nil {
		return fmt.Errorf("migration failed: %w", err)
	}

	repo := database.NewRepository(connectionPool)
	redisCache, err := cache.NewRedisCache(context.Background(), cfg.RedisAddr)
	if err != nil {
		return fmt.Errorf("connection to redis failed: %w", err)
	}

	jobsChannel := make(chan models.NotificationJob, 100)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	group, groupCtx := errgroup.WithContext(ctx)

	smtpSender := workers.NewSMTPSender(cfg)
	notifier := workers.NewNotifierWorker(smtpSender)
	group.Go(withRecover(func() error {
		return notifier.Start(groupCtx, jobsChannel)
	}))

	githubClient := gh.NewClient(cfg.GitHubToken, redisCache)

	scanner := workers.NewScannerWorker(repo, jobsChannel, githubClient)
	group.Go(withRecover(func() error {
		return scanner.StartScanner(groupCtx)
	}))

	svc := service.NewSubscriptionService(repo, cfg.BaseURL, jobsChannel, githubClient)

	router := routes.SetupRouter(repo, svc, cfg)
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

		log.Println("Shutting down HTTP server...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Printf("HTTP server shutdown error: %v", err)
		}

		log.Println("Closing jobs channel...")
		close(jobsChannel)

		return nil
	})

	log.Printf("Server started on :%s", cfg.ServerPort)

	return group.Wait()
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
