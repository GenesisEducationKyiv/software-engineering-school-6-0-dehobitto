package main

import (
	"context"
	"fmt"
	"os"

	"subber/pkg/logger"
	"subber/pkg/postgres"
	"subber/services/subscription-api/internal/config"
	"subber/services/subscription-api/internal/dbmigrations"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "subscription-api migrate error: %v\n", err)
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

	log := logger.New().WithField("service", "subscription-api-migrate")
	log.Info("migration started")

	pool, err := postgres.Connect(context.Background(), cfg.Postgres())
	if err != nil {
		log.WithError(err).Error("migration failed")
		return fmt.Errorf("connect subscription database: %w", err)
	}
	defer pool.Close()
	log.Info("database connected")

	if err := dbmigrations.Run(context.Background(), pool); err != nil {
		log.WithError(err).Error("migration failed")
		return err
	}
	log.Info("migration completed")
	return nil
}
