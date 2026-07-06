package main

import (
	"context"
	"fmt"
	"os"

	"subber/pkg/postgres"
	"subber/services/notification-service/internal/config"
	"subber/services/notification-service/internal/dbmigrations"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "notification-service migrate error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg := config.Load()
	pool, err := postgres.Connect(context.Background(), cfg.Postgres())
	if err != nil {
		return fmt.Errorf("connect notification database: %w", err)
	}
	defer pool.Close()

	return dbmigrations.Run(context.Background(), pool)
}
