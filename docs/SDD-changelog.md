# SDD Changelog

This file describes how the current implementation differs from the baseline design in `docs/SDD.md`.

The original SDD is intentionally kept unchanged as the previous architecture snapshot. This changelog records the architectural delta introduced by the microservice refactor.

## 2026-06-12 - Monolith To Microservices

### Architecture Style

**Before:** the SDD described a single Go monolith with two in-process background workers: Scanner Worker and Notifier Worker.

**Now:** Subber is a modular microservice system with three independently runnable Go services:

| Service | Responsibility |
| --- | --- |
| `subscription-api` | HTTP API, subscriptions, confirmation/unsubscribe flow, subscriber source of truth, release expansion |
| `scanner-service` | GitHub release polling, scanner watchlist, scanner-local cache, release detection |
| `notification-service` | Email delivery, delivery state, retries, idempotency |

The old in-process boundaries became process boundaries. Shared code that is not domain-specific lives in `pkg`; service-owned code lives inside each service.

### Runtime Modules

**Before:** the application entrypoint was `cmd/api-server`, and most runtime logic lived under root `internal/`.

**Now:** runtime code is split into service modules:

* `services/subscription-api`
* `services/scanner-service`
* `services/notification-service`
* `pkg`

The workspace is coordinated through `go.work`.

### Communication Between Domains

**Before:** the API, scanner, and notifier communicated inside one process. Notification jobs were passed through an in-memory channel.

**Now:** services communicate asynchronously through Kafka. Services do not call each other directly for business workflows.

Current Kafka topics:

| Topic | Purpose |
| --- | --- |
| `subber.watchlist.events` | subscription API tells scanner to start/stop watching a repository |
| `subber.release.events` | scanner publishes detected releases |
| `subber.notification.commands` | subscription API requests email delivery |
| `subber.notification.retry.1m` | first delayed notification retry |
| `subber.notification.retry.10m` | second delayed notification retry |
| `subber.notification.dlq` | exhausted notification commands |

Messages use an envelope with `event_id`, `event_type`, `occurred_at`, `source`, `correlation_id`, and `payload`.

### Reliability

**Before:** email jobs could be lost if the in-memory notification channel was full or if the process crashed before a worker handled the job.

**Now:** business events and notification commands are written through a transactional outbox and then published to Kafka by outbox relays.

The notifier processes email delivery with at-least-once semantics:

* delivery is protected by idempotency keys;
* failed sends are retried with limited retry topics;
* exhausted sends are moved to the notification DLQ.

### Data Ownership

**Before:** the SDD described one PostgreSQL database and one denormalized `subscriptions` table shared by API, scanner, and notifier logic.

**Now:** each service owns its own PostgreSQL database and schema:

| Service | Database | Owned data |
| --- | --- | --- |
| `subscription-api` | `subber_api` | subscriptions, tokens, subscription outbox |
| `scanner-service` | `subber_scanner` | scanner watchlist, scan state, scanner outbox |
| `notification-service` | `subber_notifier` | delivery state, idempotency records, notification outbox |

Services do not share tables. Cross-service data movement happens through Kafka events.

### Scanner And Cache

**Before:** scanner state and `last_seen_tag` were stored in the shared subscriptions table, and Redis was described as an application-level GitHub API cache.

**Now:** scanner state belongs to `scanner-service`.

The scanner stores only its own watchlist and scan state, claims repositories in configurable batches, and uses Redis as a scanner-local GitHub API cache. If Redis is unavailable, scanner can bypass cache and call GitHub directly.

### Notification Flow

**Before:** Notifier Worker consumed in-memory jobs and sent emails directly. SMTP failures caused dropped messages with no automatic retry.

**Now:** `notification-service` consumes Kafka notification commands, stores delivery state, deduplicates by idempotency key, retries failed sends, and records exhausted commands in DLQ.

### Logging

**Before:** ADR history included RabbitMQ/Logstash as a reliable log transport path.

**Now:** logs are best-effort operational data, not business data.

Services:

* write structured JSON logs to stdout;
* push logs to Vector by default;
* can optionally duplicate logs to a file through config, disabled by default.

Vector sends logs to Elasticsearch in batches. Kafka is not used for log transport.

### Observability

**Before:** observability was tied to the monolith and old deployment overlays.

**Now:** the microservice stack includes:

| Tool | Purpose |
| --- | --- |
| Prometheus | scrape metrics from all three services |
| Grafana | metrics dashboard |
| Elasticsearch | log storage/search |
| Kibana | log dashboard/search UI |
| Vector | log collector and Elasticsearch batch sender |

The Kibana dashboard artifact is stored at `deployments/docker/kibana/dashboards.ndjson`.

### Deployment

**Before:** local deployment used the old root Dockerfile and root `docker-compose.yml`.

**Now:** local deployment uses:

* `compose.microservices.yml` as the root compose file;
* `deployments/docker/infra.compose.yml` for infrastructure;
* service-local Dockerfiles and compose files under `services/*`.

The stack runs separate containers for each service, each service outbox relay, Kafka, three PostgreSQL databases, Redis, Vector, Elasticsearch, Kibana, Prometheus, Grafana, and Mailpit.

### Configuration

**Before:** configuration was monolith-oriented.

**Now:** runtime config is env-based and split by ownership:

* common local defaults live in `deployments/docker/env/common.env.example`;
* service-specific defaults live in each service `.env.example`;
* duplicated config parsing lives in `pkg/config`;
* service-specific config remains in each service.

YAML config files are intentionally not used for runtime service configuration.

### API And Event Contracts

**Before:** the SDD described HTTP endpoints but did not separate formal API and event contracts.

**Now:**

* HTTP API contract lives in `api/openapi/subscription-api.yaml`;
* Kafka event contract lives in `api/asyncapi/subber-events.yaml`.

### Verification

**Before:** tests targeted the monolith and old internal packages.

**Now:** verification is split across:

* unit tests for shared packages and service-owned domain logic;
* integration tests for database-backed subscription/outbox behavior;
* runtime smoke scripts for local stack readiness;
* Kafka E2E scripts for the asynchronous business flow.

Current commands are documented in `testing.md`.
