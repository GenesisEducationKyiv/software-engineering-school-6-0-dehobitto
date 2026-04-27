package database

import (
	"context"
	"fmt"
	"log"
	"os"

	"subber/config"

	"github.com/jackc/pgx/v5/pgxpool"
)

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

func Migrate(pool *pgxpool.Pool, filePath string) error {
	schema, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	_, err = pool.Exec(context.Background(), string(schema))
	if err != nil {
		return err
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
