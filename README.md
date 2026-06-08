# Subber

A service that watches GitHub repositories for new releases and notifies subscribers by email.

## Local endpoints

| Service | URL | Description |
|---------|-----|-------------|
| App | http://localhost:8080 | Main application |
| Swagger / UI | http://localhost:8080/static/index.html | Web UI |
| Prometheus metrics | http://localhost:8080/metrics | Raw metrics |
| Prometheus | http://localhost:9090 | Metrics store |
| Grafana | http://localhost:3000 | Metrics dashboards (admin / admin) |
| Kibana | http://localhost:5601 | Log dashboards |
| Elasticsearch | http://localhost:9200 | Search engine (raw API) |
| RabbitMQ management | http://localhost:15672 | Queue dashboard (guest / guest) |

## How to run

**1. Setup**
```bash
cp .env.example .env
# fill in: GITHUB_TOKEN, API_KEY, SMTP_EMAIL, SMTP_PASSWORD
```

**2. Run app only**
```bash
docker compose up --build -d
```

**3. Run app + logging stack (RabbitMQ → Logstash → Elasticsearch → Kibana)**
```bash
docker compose -f docker-compose.yml -f docker/docker-compose.logging.yml up --build -d
```

**4. Run app + metrics stack (Prometheus → Grafana)**
```bash
docker compose -f docker-compose.yml -f docker/docker-compose.observability.yml up --build -d
```

**5. Run app + logging + metrics stack**
```bash
docker compose -f docker-compose.yml -f docker/docker-compose.logging.yml -f docker/docker-compose.observability.yml up --build -d
```

**6. Run app + logging + load test**
```bash
# start app and logging stack first
docker compose -f docker-compose.yml -f docker/docker-compose.logging.yml up --build -d

# then run k6 (exits automatically when done)
docker compose -f docker-compose.yml -f docker/docker-compose.loadtest.yml run --rm k6
```

**7. Verify full observability pipeline**
```bash
# Windows PowerShell
.\scripts\observability-smoke.ps1

# Linux / macOS / Git Bash
sh scripts/observability-smoke.sh
```

**Stop everything**
```bash
docker compose -f docker-compose.yml -f docker/docker-compose.logging.yml down
```

---

## How it works

When a user subscribes to a repository, Subber saves the subscription as unconfirmed and sends a confirmation email. Once confirmed, a background scanner polls GitHub every 30 seconds for new release tags. When a new tag is detected, the notifier worker sends an email to every confirmed subscriber of that repository.

```
POST /api/subscribe
    │
    ▼
    save subscription (unconfirmed)
    send confirmation email
    │
    ▼ (user clicks link)
GET /api/confirm/:token
    │
    ▼
    mark subscription confirmed
    │
ScannerWorker (every 30s)
    │
    ▼
    poll GitHub API → compare tag → update DB → enqueue jobs
    │
NotifierWorker
    │
    ▼
    send emails via SMTP
```

**Storage**
- PostgreSQL - subscriptions (email, repo, token, confirmed, last seen tag)
- Redis - GitHub API response cache (10-minute TTL, reduces API rate limit pressure)

## API

Protected endpoints require `X-API-Key` header. If `API_KEY` is empty in config, auth is skipped.

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `POST` | `/api/subscribe` | +| Subscribe email to a repo |
| `GET` | `/api/subscriptions/` | + | List confirmed subscriptions for an email |
| `GET` | `/api/confirm/:token` | -| Confirm a subscription |
| `GET` | `/api/unsubscribe/:token` | -| Unsubscribe |
| `GET` | `/metrics` | - | Prometheus metrics |

**Subscribe request body:**
```json
{ "email": "user@example.com", "repo": "owner/repository" }
```

## Quick start

```bash
cp .env.example .env
# fill in SMTP_EMAIL, SMTP_PASSWORD, GITHUB_TOKEN, API_KEY

# build and start everything in the background
docker compose up --build -d

# check logs
docker compose logs -f subber
```

The app will be available at `http://localhost:8080`.

To stop:
```bash
docker compose down
```

## Environment variables

| Variable | Default | Description |
|----------|---------|-------------|
| `DB_HOST` | `localhost` | PostgreSQL host |
| `DB_PORT` | `5432` | PostgreSQL port |
| `DB_USER` | `postgres` | PostgreSQL user |
| `DB_PASSWORD` | `postgres` | PostgreSQL password |
| `DB_NAME` | `db` | Database name |
| `PORT` | `8080` | HTTP server port |
| `REDIS_ADDR` | `redis:6379` | Redis address |
| `GITHUB_TOKEN` | — | GitHub personal access token (increases rate limit) |
| `GITHUB_BASE_URL` | `https://api.github.com` | GitHub API base URL, overridden by E2E tests |
| `SMTP_HOST` | `smtp.gmail.com` | SMTP server host |
| `SMTP_PORT` | `587` | SMTP server port |
| `SMTP_EMAIL` | — | Sender email address |
| `SMTP_PASSWORD` | — | SMTP password / app password |
| `API_KEY` | — | API key for protected endpoints (leave empty to disable auth) |
| `BASE_URL` | `http://localhost:8080` | Used in confirmation email links |
| `GIN_MODE` | — | Set to `release` to silence Gin debug output |
| `LOG_LEVEL` | `info` | Structured log level |
| `LOG_FILE` | — | Optional local log file path, opened with `0600` permissions |
| `RABBITMQ_URL` | — | Enables async log publishing to RabbitMQ when set |

## Observability

Detailed docs:

- [Observability workflow](docs/observability.md)
- [Logging stack and format](docs/logging.md)

**Logs**

Application logs are JSON. Each component receives an injected logger with a `component` field. HTTP logs include `request_id`, `method`, `route`, `status_code`, `duration_ms`, `has_query`, `user_agent`, and `client_ip_hash`. Email addresses are logged as `email_hash`; raw tokens, raw query strings, raw emails, and raw IP addresses are not logged.

When `RABBITMQ_URL` is set, logs are published asynchronously to RabbitMQ queue `logs`. Logstash consumes the queue and indexes documents into Elasticsearch as `subber-logs-YYYY.MM.dd`.

Elasticsearch applies a `subber-logs-7d` lifecycle policy through the logging stack setup container. Subber log indices are retained for 7 days.

**Metrics**

The app exposes Prometheus metrics on `/metrics`. The core RED metrics are:

- `http_requests_total{method,route,status_code}`
- `http_request_duration_seconds{method,route}`
- `emails_sent_total`
- `emails_failed_total`
- `scan_cycles_total`
- `log_entries_enqueued_total`
- `log_entries_dropped_total`
- `log_entries_published_total`
- `log_publish_errors_total`

Grafana is provisioned automatically with Prometheus as the datasource and a `Subber RED` dashboard.

Prometheus alert rules are provisioned from `docker/prometheus/rules/subber-alerts.yml`. The local Docker Compose profile keeps Grafana, Prometheus, Kibana, Elasticsearch, and RabbitMQ management ports published for development access. Do not expose those ports directly in production; put them behind private networking and authentication.

## Development

**Run tests:**
```bash
go test ./...
```

**Run linter:**
```bash
golangci-lint run
```

**Run locally (without Docker):**
```bash
# start dependencies only
docker compose up postgres redis -d

# set env vars, then
go run ./cmd/api-server/
```

## CI pipeline

Workflows run on every push and pull request to `main`:

| Workflow | Trigger | What it does |
|----------|---------|--------------|
| **Build** | push / PR → `main` | `go build ./...` — ensures the project compiles |
| **Test: Unit** | push / PR → `main` | `go test ./...` — runs unit tests |
| **Test: Integration** | push / PR → `main` | `go test -tags integration ./tests/integration/...` — runs PostgreSQL-backed integration tests |
| **Test: E2E** | push / PR → `main` | Docker Compose launches frontend, backend, PostgreSQL, Redis, and fake GitHub API; Playwright tests the real FE + BE flow |
| **Lint** | push / all PRs | `golangci-lint run` — static analysis and formatting checks |

All workflows must pass before a PR can be merged.
