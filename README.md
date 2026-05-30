# Subber

A service that watches GitHub repositories for new releases and notifies subscribers by email.

## Local endpoints

| Service | URL | Description |
|---------|-----|-------------|
| App | http://localhost:8080 | Main application |
| Swagger / UI | http://localhost:8080/static/index.html | Web UI |
| Prometheus metrics | http://localhost:8080/metrics | Raw metrics |
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

**4. Run app + logging + load test**
```bash
# start app and logging stack first
docker compose -f docker-compose.yml -f docker/docker-compose.logging.yml up --build -d

# then run k6 (exits automatically when done)
docker compose -f docker-compose.yml -f docker/docker-compose.loadtest.yml run --rm k6
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
| `SMTP_HOST` | `smtp.gmail.com` | SMTP server host |
| `SMTP_PORT` | `587` | SMTP server port |
| `SMTP_EMAIL` | — | Sender email address |
| `SMTP_PASSWORD` | — | SMTP password / app password |
| `API_KEY` | — | API key for protected endpoints (leave empty to disable auth) |
| `BASE_URL` | `http://localhost:8080` | Used in confirmation email links |
| `GIN_MODE` | — | Set to `release` to silence Gin debug output |

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

Three workflows run on every push and pull request to `main`:

| Workflow | Trigger | What it does |
|----------|---------|--------------|
| **Build** | push / PR → `main` | `go build ./...` — ensures the project compiles |
| **Test** | push / PR → `main` | `go test ./...` — runs the test suite |
| **Lint** | push / all PRs | `golangci-lint run` — static analysis and formatting checks |

All three must pass before a PR can be merged.
