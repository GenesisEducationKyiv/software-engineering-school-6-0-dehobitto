//go:build integration

package integration

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

var sharedPool *pgxpool.Pool

func TestMain(m *testing.M) {
	os.Exit(run(m))
}

func run(m *testing.M) int {
	ctx := context.Background()

	pgContainer, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		panic(err)
	}
	defer pgContainer.Terminate(ctx) //nolint:errcheck

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		panic(err)
	}

	sharedPool, err = pgxpool.New(ctx, connStr)
	if err != nil {
		panic(err)
	}
	defer sharedPool.Close()

	return m.Run()
}

func truncateOutbox(t *testing.T) {
	t.Helper()
	if _, err := sharedPool.Exec(context.Background(), "TRUNCATE TABLE outbox_events"); err != nil {
		t.Fatalf("truncate outbox_events: %v", err)
	}
}
