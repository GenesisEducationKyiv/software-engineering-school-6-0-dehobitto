// Package database provides PostgreSQL connection management and data access.
package database

import (
	"context"
	"embed"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"subber/internal/config"
	"subber/internal/logger"
)

var dbLog = logger.New().WithField("component", "database")

//go:embed schemas.sql
var schemaFS embed.FS

func Connect(cfg *config.Config) (*pgxpool.Pool, error) {
	dsn := getDSN(cfg)

	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		return nil, err
	}

	if err := pool.Ping(context.Background()); err != nil {
		return nil, err
	}

	dbLog.Info("database connection established")
	return pool, nil
}

func Migrate(pool *pgxpool.Pool) error {
	schema, err := schemaFS.ReadFile("schemas.sql")
	if err != nil {
		return fmt.Errorf("read embedded schema: %w", err)
	}

	_, err = pool.Exec(context.Background(), string(schema))
	if err != nil {
		return fmt.Errorf("apply schema: %w", err)
	}

	dbLog.Info("migrations applied successfully")
	return nil
}

func getDSN(cfg *config.Config) string {
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		cfg.DBHost,
		cfg.DBPort,
		cfg.DBUser,
		cfg.DBPassword,
		cfg.DBName,
	)
}
