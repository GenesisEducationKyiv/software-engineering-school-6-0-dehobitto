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
	"github.com/prometheus/client_golang/prometheus"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"subber/pkg/contracts"
	"subber/services/subscription-api/internal/httpapi"
	"subber/services/subscription-api/internal/subscription"
)

var sharedPool *pgxpool.Pool

func TestMain(m *testing.M) {
	os.Exit(run(m))
}

func run(m *testing.M) int {
	ctx := context.Background()

	pgContainer, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("subscription_api_test"),
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

	if err := subscription.Migrate(ctx, sharedPool); err != nil {
		panic(err)
	}

	return m.Run()
}

type testEnv struct {
	router *gin.Engine
	pool   *pgxpool.Pool
}

func newTestEnv(t *testing.T, fake gitHubFake) *testEnv {
	t.Helper()
	if _, err := sharedPool.Exec(context.Background(), "TRUNCATE TABLE subscriptions, outbox_events"); err != nil {
		t.Fatalf("truncate tables: %v", err)
	}

	repo := subscription.NewRepository(sharedPool)
	publisher := subscription.NewOutboxNotificationPublisher(sharedPool, "http://localhost:8080")
	svc := subscription.NewService(repo, publisher, fake, nil)

	gin.SetMode(gin.TestMode)
	router := httpapi.SetupRouter(httpapi.RouterDeps{
		APIKey:   "test-key",
		Repo:     repo,
		Service:  svc,
		Gatherer: prometheus.NewRegistry(),
	})

	return &testEnv{router: router, pool: sharedPool}
}

func (e *testEnv) subscribe(t *testing.T, email, repo string, apiKey string) *httptest.ResponseRecorder {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"email": email, "repo": repo})
	req := httptest.NewRequest(http.MethodPost, "/api/subscribe", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		req.Header.Set("X-API-Key", apiKey)
	}
	res := httptest.NewRecorder()
	e.router.ServeHTTP(res, req)
	return res
}

func (e *testEnv) confirm(t *testing.T, token string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/confirm/"+token, nil)
	res := httptest.NewRecorder()
	e.router.ServeHTTP(res, req)
	return res
}

func (e *testEnv) unsubscribe(t *testing.T, token string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/unsubscribe/"+token, nil)
	res := httptest.NewRecorder()
	e.router.ServeHTTP(res, req)
	return res
}

func (e *testEnv) getSubscriptions(t *testing.T, email, apiKey string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/subscriptions/?email="+email, nil)
	if apiKey != "" {
		req.Header.Set("X-API-Key", apiKey)
	}
	res := httptest.NewRecorder()
	e.router.ServeHTTP(res, req)
	return res
}

func subscriptionExists(t *testing.T, email, repo string) bool {
	t.Helper()
	var exists bool
	err := sharedPool.QueryRow(context.Background(),
		`SELECT EXISTS(SELECT 1 FROM subscriptions WHERE email=$1 AND repo=$2)`,
		email,
		repo,
	).Scan(&exists)
	if err != nil {
		t.Fatalf("subscriptionExists query: %v", err)
	}
	return exists
}

func isConfirmed(t *testing.T, email, repo string) bool {
	t.Helper()
	var confirmed bool
	err := sharedPool.QueryRow(context.Background(),
		`SELECT confirmed FROM subscriptions WHERE email=$1 AND repo=$2`,
		email,
		repo,
	).Scan(&confirmed)
	if err != nil {
		t.Fatalf("isConfirmed query: %v", err)
	}
	return confirmed
}

func tokenForSubscription(t *testing.T, email, repo string) string {
	t.Helper()
	var token string
	err := sharedPool.QueryRow(context.Background(),
		`SELECT token FROM subscriptions WHERE email=$1 AND repo=$2`,
		email,
		repo,
	).Scan(&token)
	if err != nil {
		t.Fatalf("tokenForSubscription query: %v", err)
	}
	return token
}

func outboxCount(t *testing.T, eventType, topic string) int {
	t.Helper()
	var count int
	err := sharedPool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM outbox_events WHERE event_type=$1 AND topic=$2`,
		eventType,
		topic,
	).Scan(&count)
	if err != nil {
		t.Fatalf("outbox count query: %v", err)
	}
	return count
}

type gitHubFake struct {
	existsErr error
	tag       string
	tagErr    error
}

func (g gitHubFake) CheckIfRepoExists(context.Context, string) error {
	return g.existsErr
}

func (g gitHubFake) GetLatestTag(context.Context, string) (string, error) {
	return g.tag, g.tagErr
}

func githubError(status int) error {
	switch status {
	case http.StatusNotFound:
		return subscription.ErrGitHubNotFound
	case http.StatusTooManyRequests:
		return subscription.ErrGitHubAPILimit
	default:
		return fmt.Errorf("github error: %d", status)
	}
}

func notificationCommandCount(t *testing.T) int {
	t.Helper()
	return outboxCount(t, contracts.EventNotificationRequested, contracts.TopicNotificationCommands)
}
