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

	connectionPool, err := database.Connect(cfg)
	if err != nil {
		log.Fatal(err)
	}
	defer connectionPool.Close()

	err = database.Migrate(connectionPool, cfg.SchemasPath)
	if err != nil {
		log.Fatalf("Migration failed: %v", err)
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
		log.Fatalf("Failed to start server: %v", err)
	}
}
