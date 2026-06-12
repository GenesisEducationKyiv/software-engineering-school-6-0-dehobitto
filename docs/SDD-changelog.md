# SDD Changelog

This file records architecture changes made after the original SDD. The original `docs/SDD.md` is intentionally kept as the baseline document.

## 2026-06-12 - Modular Microservice Refactor

### Service Boundaries

* Extracted the monolith into three service modules:
  * `services/subscription-api` owns HTTP API, subscriptions, confirmation/unsubscribe tokens, subscriber source of truth, and release expansion.
  * `services/scanner-service` owns GitHub release polling, scanner watchlist, scanner-local GitHub cache, and release detection.
  * `services/notification-service` owns email delivery, delivery state, retries, DLQ handling, and idempotency.
* Added `go.work` for the root workspace.
* Added shared `pkg` modules for contracts, logging, metrics, Kafka, outbox, config, request id, and PostgreSQL helpers.
* Added `api/openapi/subscription-api.yaml` for HTTP API contracts.
* Added `api/asyncapi/subber-events.yaml` for Kafka event contracts.

### Messaging

* Replaced in-process notification jobs with Kafka-based messaging.
* Added Kafka topics:
  * `subber.watchlist.events`
  * `subber.release.events`
  * `subber.notification.commands`
  * `subber.notification.retry.1m`
  * `subber.notification.retry.10m`
  * `subber.notification.dlq`
* Added event envelope fields: `event_id`, `event_type`, `occurred_at`, `source`, `correlation_id`, and `payload`.
* Added Kafka keys:
  * repo key for watchlist and release events;
  * email hash key for notification commands.
* Added transactional outbox relays per service database.

### Data Ownership

* Split storage into separate PostgreSQL containers/databases:
  * `postgres-api` / `subber_api`
  * `postgres-scanner` / `subber_scanner`
  * `postgres-notifier` / `subber_notifier`
* `subscription-api` no longer shares tables with scanner or notifier.
* `scanner-service` stores its own watchlist with repo scan state.
* `notification-service` stores delivery/idempotency state.
* Scanner Redis cache moved into `scanner-service`; it is no longer shared as generic app cache.

### Runtime And Deployment

* Added root `compose.microservices.yml`.
* Added service-local Dockerfiles and compose files:
  * `services/subscription-api/Dockerfile`
  * `services/subscription-api/compose.yml`
  * `services/scanner-service/Dockerfile`
  * `services/scanner-service/compose.yml`
  * `services/notification-service/Dockerfile`
  * `services/notification-service/compose.yml`
* Added `deployments/docker/infra.compose.yml` for infrastructure.
* Removed old root monolith Dockerfile, old root compose file, and old compose overlays.
* Removed old monolith entrypoint `cmd/api-server`.
* Removed old root `internal/` implementation after logic was moved into service modules.

### Logging And Observability

* Replaced RabbitMQ/Logstash log transport with Vector sidecar log shipping.
* Kafka is used for domain events/jobs, not logs.
* Services write structured JSON logs to stdout.
* Services push logs to Vector by default.
* Optional file log duplication can be enabled through config, but is disabled by default.
* Vector batches logs to Elasticsearch by size/time.
* Restored Kibana as the log dashboard/search UI and moved the dashboard artifact to `deployments/docker/kibana/dashboards.ndjson`.
* Prometheus scrapes metrics from all three services.
* Grafana is provisioned with a Prometheus datasource and `Subber Overview` dashboard.

### Configuration

* Runtime configuration remains env-based; YAML config files are not used.
* Added common env defaults in `deployments/docker/env/common.env.example`.
* Added service-specific env examples:
  * `services/subscription-api/.env.example`
  * `services/scanner-service/.env.example`
  * `services/notification-service/.env.example`
* Added `.env` ignore rules.
* Moved duplicated common config parsing into `pkg/config`; service-specific config stays inside each service.

### Tests And Verification

* Added/updated unit tests for service modules.
* Added integration tests for database-backed subscription and outbox behavior.
* Added Kafka E2E scripts:
  * `scripts/kafka-e2e.ps1`
  * `scripts/kafka-e2e.sh`
* Added runtime smoke scripts:
  * `scripts/runtime-smoke.ps1`
  * `scripts/runtime-smoke.sh`
* Removed old `scripts/test.*` and `scripts/observability-smoke.*`.
* Updated `testing.md` with current verification commands.

### Documentation

* Added ADRs for service boundaries, Kafka, Vector logging, and modular microservice boundaries.
* Marked the old RabbitMQ/Logstash log transport ADR as superseded.
* Updated `README.md`, `docs/logging.md`, and `docs/observability.md` to describe the target runtime.
