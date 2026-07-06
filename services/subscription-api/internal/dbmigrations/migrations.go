package dbmigrations

import (
	"context"
	"embed"

	"github.com/jackc/pgx/v5/pgxpool"

	runner "subber/pkg/migrations"
	outboxmigrations "subber/pkg/outbox/migrations"
)

//go:embed *.sql
var files embed.FS

func Run(ctx context.Context, pool *pgxpool.Pool) error {
	list, err := runner.Load(files, ".")
	if err != nil {
		return err
	}
	if err := runner.Run(ctx, pool, "subscription-api", list); err != nil {
		return err
	}
	return outboxmigrations.Run(ctx, pool)
}
