package migrations

import (
	"context"
	"embed"

	"github.com/jackc/pgx/v5/pgxpool"

	runner "subber/pkg/migrations"
)

//go:embed *.sql
var files embed.FS

func Run(ctx context.Context, pool *pgxpool.Pool) error {
	list, err := runner.Load(files, ".")
	if err != nil {
		return err
	}
	return runner.Run(ctx, pool, "outbox", list)
}
