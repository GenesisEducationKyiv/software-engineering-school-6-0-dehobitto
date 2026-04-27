package main

import (
	"log"
	"subber/config"
	"subber/infra/cache"
	"subber/infra/database"
	"subber/routes"
	"subber/workers"
)

func main() {
	cfg := config.LoadConfig()

	pool, err := database.Connect(cfg)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	err = database.Migrate(pool, "infra/database/schemas.sql")
	if err != nil {
		log.Fatalf("Migration failed: %v", err)
	}

	repo := database.NewRepository(pool)

	rc := cache.NewRedisCache(cfg.RedisAddr)

	jobsChannel := make(chan workers.NotificationJob, 100)

	notifier := workers.NewNotifierWorker(cfg)
	go notifier.Start(jobsChannel)

	scanner := workers.NewScannerWorker(repo, cfg, jobsChannel, rc)
	go scanner.StartScanner()

	r := routes.SetupRouter(repo, cfg, jobsChannel, rc)

	if err = r.Run(":" + cfg.ServerPort); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
