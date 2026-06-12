# Subber

Subber watches GitHub repositories for new releases and notifies subscribers by email.

The target runtime is split into three Go services:

* `services/subscription-api` - HTTP API, subscriptions, confirmation/unsubscribe flow, outbox publishing.
* `services/scanner-service` - scanner watchlist, GitHub release polling, Redis-backed GitHub cache.
* `services/notification-service` - email delivery, idempotency, retries, dead-letter handling.

Shared code lives in `pkg`. Service-specific code stays inside each service module.

## Requirements

* Go workspace support via `go.work`.
* Docker Compose `2.20.3` or newer for `include`.

## Local Endpoints

| Service | URL | Description |
| --- | --- | --- |
| Subscription API | http://localhost:8080 | HTTP API |
| Subscription API metrics | http://localhost:8080/metrics | Prometheus metrics |
| Scanner metrics | http://localhost:8081/metrics | Prometheus metrics |
| Notification metrics | http://localhost:8082/metrics | Prometheus metrics |
| Kafka | localhost:9092 | Domain event bus |
| Redis | localhost:6379 | Scanner GitHub cache |
| Elasticsearch | http://localhost:9200 | Log search |
| Vector | http://localhost:8686 | Log sidecar input |
| Prometheus | http://localhost:9090 | Metrics scraping |
| Grafana | http://localhost:3000 | Metrics dashboards |
| Mailpit | http://localhost:8025 | Local email inspection |

## Run Microservices

```bash
docker compose -f compose.microservices.yml up --build -d
```

Stop:

```bash
docker compose -f compose.microservices.yml down
```

The root compose file includes service-local compose files from `services/*/compose.yml` and infrastructure from `deployments/docker/infra.compose.yml`.

## Local Configuration

Each service has its own `.env.example`:

* [deployments/docker/env/common.env.example](deployments/docker/env/common.env.example)
* [services/subscription-api/.env.example](services/subscription-api/.env.example)
* [services/scanner-service/.env.example](services/scanner-service/.env.example)
* [services/notification-service/.env.example](services/notification-service/.env.example)

The compose stack loads the common env file first and then the service-specific env file. Common infrastructure settings such as DB user/password, Kafka brokers, GitHub base URL, and logging defaults live in `deployments/docker/env/common.env.example`. Service-owned settings such as DB host/name, ports, Redis, SMTP, and retry policy live next to each service.

The compose stack provides local defaults for databases, Kafka, Redis, Vector, Prometheus, Grafana, and Mailpit.

Useful local env values:

```bash
API_KEY=dev-api-key
GITHUB_TOKEN=...
LOG_SIDECAR_ENABLED=false
LOG_FILE=/tmp/subber.log
SMTP_HOST=mailpit
SMTP_PORT=1025
SMTP_EMAIL=
SMTP_PASSWORD=
```

Runtime configuration is env-based. YAML config files are intentionally not used; secrets and environment-specific values should stay outside committed files.

## Flow

```text
POST /api/subscribe
  -> subscription-api saves unconfirmed subscription
  -> subscription-api writes NotificationSendRequested to outbox
  -> outbox relay publishes to Kafka
  -> notification-service sends confirmation email
  -> user confirms subscription
  -> subscription-api writes RepoWatchStartRequested
  -> scanner-service stores repo in its watchlist
  -> scanner-service detects ReleaseDetected
  -> subscription-api expands release to NotificationSendRequested per subscriber
  -> notification-service sends release emails
```

## API

Protected endpoints require `X-API-Key`. If `API_KEY` is empty, auth is skipped.

| Method | Path | Auth | Description |
| --- | --- | --- | --- |
| `POST` | `/api/subscribe` | yes | Subscribe email to a repo |
| `GET` | `/api/subscriptions/` | yes | List confirmed subscriptions for an email |
| `GET` | `/api/confirm/:token` | no | Confirm a subscription |
| `GET` | `/api/unsubscribe/:token` | no | Unsubscribe |
| `GET` | `/metrics` | no | Prometheus metrics |

OpenAPI is stored in [api/openapi/subscription-api.yaml](api/openapi/subscription-api.yaml).

Kafka contracts are stored in [api/asyncapi/subber-events.yaml](api/asyncapi/subber-events.yaml).

## Storage

* `subscription-api` owns its PostgreSQL database with subscriptions and outbox.
* `scanner-service` owns its PostgreSQL database with the scanner watchlist.
* `notification-service` owns its PostgreSQL database with delivery state.
* Redis is a scanner-local dependency for GitHub API caching.
* Kafka carries domain events and notification jobs.

## Logging

Services always write structured JSON logs to stdout and push logs to Vector by default.

Default logging config:

```text
LOG_SIDECAR_ENABLED=true
LOG_SIDECAR_URL=http://vector:8686
LOG_FILE=
```

`LOG_FILE` is empty by default. Set it only when a service must also duplicate logs to a local file.

Kafka, RabbitMQ, and Logstash are not used for log transport in the target architecture.

More details: [docs/logging.md](docs/logging.md).

## Development

Run all tests:

```bash
go test ./pkg/... ./services/subscription-api/... ./services/scanner-service/... ./services/notification-service/...
```

Run integration tests:

```bash
go test -tags integration ./tests/integration/... ./services/subscription-api/...
```

Validate compose:

```bash
docker compose -f compose.microservices.yml config --quiet
```

Runtime smoke:

```bash
pwsh scripts/runtime-smoke.ps1
# or
sh scripts/runtime-smoke.sh
```

Runtime smoke is a fast "is the stack alive?" check. It validates compose config, HTTP endpoints, all three metrics endpoints, Prometheus targets, Grafana datasource/dashboard provisioning, Mailpit, Elasticsearch, required Kafka topics, and the Vector-to-Elasticsearch log path.

Start the stack from the smoke script when needed:

```bash
pwsh scripts/runtime-smoke.ps1 -StartStack -Build
# or
START_STACK=true BUILD=true sh scripts/runtime-smoke.sh
```

Kafka end-to-end flow:

```bash
pwsh scripts/kafka-e2e.ps1
# or
sh scripts/kafka-e2e.sh
```

Kafka E2E verifies the business chain through the message bus: subscribe, confirm, watchlist event consumed by scanner, manual `ReleaseDetected` published to Kafka, release expanded by `subscription-api`, and release notification sent by `notification-service`.

Mailpit UI is available at http://localhost:8025 for local confirmation and release emails.

## Load Test

```bash
docker run --rm -i \
  -e BASE_URL=http://host.docker.internal:8080 \
  -e API_KEY=dev-api-key \
  -v ${PWD}/scripts:/scripts \
  grafana/k6 run /scripts/loadtest.js
```

The load test exercises subscribe, list subscriptions, confirm, and unsubscribe paths.

When `k6` runs in Docker, `host.docker.internal` reaches the API running on the host machine.

Keep the microservice stack running while the load test executes. Prometheus and Grafana can be used to inspect service metrics during the run.

## Legacy Cleanup

The old monolith entrypoint and root `internal/` packages have been removed from the target codebase. Runtime work now lives under `services/`, shared infrastructure under `pkg/`, and API contracts under `api/`.
