//go:build integration

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"subber/internal/config"
	gh "subber/internal/github"
	"subber/internal/infra/database"
	"subber/internal/models"
	"subber/internal/routes"
	"subber/internal/service"
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

func newTestEnv(t *testing.T, fake gitHubFake) *testEnv {
	t.Helper()

	_, err := sharedPool.Exec(context.Background(), "TRUNCATE TABLE subscriptions")
	if err != nil {
		t.Fatalf("truncate: %v", err)
	}

	dbRepo := database.NewRepository(sharedPool)
	cfg := &config.Config{BaseURL: "http://localhost", APIKey: "test-key"}
	jobs := make(chan models.NotificationJob, 100)

	svc := service.NewSubscriptionService(dbRepo, cfg.BaseURL, jobs, fake, service.RealUUIDGenerator)

	gin.SetMode(gin.TestMode)
	router := routes.SetupRouter(dbRepo, svc, cfg)

	return &testEnv{router: router, pool: sharedPool}
}

// subscriptionExists checks if a subscription row exists in DB.
func subscriptionExists(t *testing.T, pool *pgxpool.Pool, email, repo string) bool {
	t.Helper()
	var exists bool
	err := pool.QueryRow(context.Background(),
		`SELECT EXISTS(SELECT 1 FROM subscriptions WHERE email=$1 AND repo=$2)`,
		email, repo,
	).Scan(&exists)
	if err != nil {
		t.Fatalf("subscriptionExists query: %v", err)
	}
	return exists
}

// isConfirmed checks if a subscription is confirmed in DB.
func isConfirmed(t *testing.T, pool *pgxpool.Pool, email, repo string) bool {
	t.Helper()
	var confirmed bool
	err := pool.QueryRow(context.Background(),
		`SELECT confirmed FROM subscriptions WHERE email=$1 AND repo=$2`,
		email, repo,
	).Scan(&confirmed)
	if err != nil {
		t.Fatalf("isConfirmed query: %v", err)
	}
	return confirmed
}

// tokenForSubscription returns the token stored for a subscription.
func tokenForSubscription(t *testing.T, pool *pgxpool.Pool, email, repo string) string {
	t.Helper()
	var token string
	err := pool.QueryRow(context.Background(),
		`SELECT token FROM subscriptions WHERE email=$1 AND repo=$2`,
		email, repo,
	).Scan(&token)
	if err != nil {
		t.Fatalf("tokenForSubscription query: %v", err)
	}
	return token
}

func (e *testEnv) subscribe(t *testing.T, email, repo string) *httptest.ResponseRecorder {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"email": email, "repo": repo})
	req := httptest.NewRequest(http.MethodPost, "/api/subscribe", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "test-key")
	w := httptest.NewRecorder()
	e.router.ServeHTTP(w, req)
	return w
}

func (e *testEnv) confirm(t *testing.T, token string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/confirm/"+token, nil)
	w := httptest.NewRecorder()
	e.router.ServeHTTP(w, req)
	return w
}

func (e *testEnv) unsubscribe(t *testing.T, token string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/unsubscribe/"+token, nil)
	w := httptest.NewRecorder()
	e.router.ServeHTTP(w, req)
	return w
}

func (e *testEnv) getSubscriptions(t *testing.T, email, apiKey string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/subscriptions/?email="+email, nil)
	if apiKey != "" {
		req.Header.Set("X-API-Key", apiKey)
	}
	w := httptest.NewRecorder()
	e.router.ServeHTTP(w, req)
	return w
}

// gitHubFake satisfies the service.GitHubClient interface.
type gitHubFake struct{ repoStatus int }

func (g gitHubFake) CheckIfRepoExists(_ context.Context, _ string) error {
	switch g.repoStatus {
	case http.StatusOK:
		return nil
	case http.StatusNotFound:
		return gh.ErrNotFound
	case http.StatusTooManyRequests:
		return gh.ErrRateLimit
	default:
		return fmt.Errorf("github error: %d", g.repoStatus)
	}
}

func (gitHubFake) GetLatestTag(_ context.Context, _ string) (string, error) {
	return "", nil
}
