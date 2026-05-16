//go:build integration

package integration

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"subber/internal/config"
	"subber/internal/infra/cache"
	"subber/internal/infra/database"
	"subber/internal/routes"
	"subber/internal/service"
	"subber/internal/workers"
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

	if err := database.Migrate(sharedPool); err != nil {
		panic(err)
	}

	return m.Run()
}

// testEnv holds everything needed to make HTTP requests against a live router.
type testEnv struct {
	router *gin.Engine
	pool   *pgxpool.Pool
}

func newTestEnv(t *testing.T, gh gitHubFake) *testEnv {
	t.Helper()

	_, err := sharedPool.Exec(context.Background(), "TRUNCATE TABLE subscriptions")
	if err != nil {
		t.Fatalf("truncate: %v", err)
	}

	dbRepo := database.NewRepository(sharedPool)
	cfg := &config.Config{BaseURL: "http://localhost", APIKey: "test-key"}
	jobs := make(chan workers.NotificationJob, 100)

	svc := service.NewSubscriptionService(dbRepo, cfg, jobs, nil, gh, service.RealUUIDGenerator)

	gin.SetMode(gin.TestMode)
	router := routes.SetupRouter(dbRepo, svc, cfg)

	return &testEnv{router: router, pool: sharedPool}
}

// gitHubFake satisfies the unexported service.gitHubClient interface.
type gitHubFake struct{ repoStatus int }

func (g gitHubFake) CheckIfRepoExists(_ context.Context, _, _ string) (*http.Response, error) {
	rec := httptest.NewRecorder()
	rec.WriteHeader(g.repoStatus)
	return rec.Result(), nil
}

func (gitHubFake) GetLatestTag(_ context.Context, _, _ string, _ cache.Cache) (string, error) {
	return "", nil
}
