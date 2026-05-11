package database

import (
	"context"
	"embed"
	"fmt"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"

	"subber/internal/config"
)

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

	log.Println("Database connection established")
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

	log.Println("Migrations applied successfully!")
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
