package main

import (
	"fmt"
	"log"
	"subber/config"
	"subber/infra/cache"
	"subber/infra/database"
	"subber/routes"
	"subber/workers"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("App error: %v", err)
	}
}

func run() error {
	cfg := config.LoadConfig()

	connectionPool, err := database.Connect(cfg)
	if err != nil {
		return fmt.Errorf("connection to database failed: %w", err)
	}
	defer connectionPool.Close()

	err = database.Migrate(connectionPool, cfg.SchemasPath)
	if err != nil {
		return fmt.Errorf("migration failed: %w", err)
	}

	repo := database.NewRepository(connectionPool)
	redisCache := cache.NewRedisCache(cfg.RedisAddr)

	jobsChannel := make(chan workers.NotificationJob, 100)

	notifier := workers.NewNotifierWorker(cfg)
	go notifier.Start(jobsChannel)

	scanner := workers.NewScannerWorker(repo, cfg, jobsChannel, redisCache)
	go scanner.StartScanner()

	router := routes.SetupRouter(repo, cfg, jobsChannel, redisCache)

	if err = router.Run(":" + cfg.ServerPort); err != nil {
		return fmt.Errorf("failed to start server: %w", err)
	}

	return nil
}
