## ADR-008: Define modular service boundaries for Subber

**Status:** Accepted

**Date:** 2026-06-11

**Author:** Oleksandr Makarov

> Builds on ADR-005, ADR-006, and ADR-007.

## Context

Subber started as one Go application that handled HTTP subscriptions, GitHub scanning, email delivery, caching, logging, and background workers in one runtime.

The target architecture must:

* clearly separate domains and module responsibilities;
* move at least one domain into a separate microservice;
* avoid direct runtime coupling between services;
* preserve existing subscription, release detection, and notification behavior;
* keep logging and observability separate from business messaging.

## Decision

Subber is split into three service modules:

* `subscription-api`
* `scanner-service`
* `notification-service`

Shared infrastructure and contracts live in `pkg` and `api`. Service-local implementation stays inside each service directory.

## Domain Boundaries

### Subscription API

`subscription-api` owns:

* HTTP API;
* subscription lifecycle;
* confirmation and unsubscribe tokens;
* subscriber data;
* expansion of `ReleaseDetected` events into `NotificationSendRequested` commands;
* outbox rows for subscription-owned events.

It does not scan GitHub and does not send emails directly.

### Scanner Service

`scanner-service` owns:

* scanner watchlist;
* GitHub release polling;
* GitHub API cache;
* release comparison;
* `ReleaseDetected` events.

It does not know subscriber emails and does not create email jobs directly.

### Notification Service

`notification-service` owns:

* email delivery;
* delivery state;
* idempotency by notification key;
* retry handling;
* DLQ publishing for exhausted attempts.

It does not read subscriptions and does not scan GitHub.

### Shared Packages

`pkg` contains only shared infrastructure and contracts needed by multiple services:

* Kafka client wrapper;
* transactional outbox;
* structured logger;
* environment parsing;
* PostgreSQL connection helper;
* request id helper;
* domain message contracts.

Domain-specific code such as scanner cache, email sender, email hashing, HTTP routing, and GitHub clients remains service-local.

## Communication

Services communicate through Kafka topics documented in AsyncAPI.

The main flows are:

```text
subscription-api -> RepoWatchStartRequested / RepoWatchStopRequested -> scanner-service
scanner-service -> ReleaseDetected -> subscription-api
subscription-api -> NotificationSendRequested -> notification-service
notification-service -> retry topics / DLQ
```

Services do not call each other directly for asynchronous business workflows.

Outgoing events are written through transactional outbox tables and published by service-local relay processes.

## Data Ownership

Each service owns its database:

* `postgres-api` for subscriptions and subscription outbox;
* `postgres-scanner` for scanner watchlist, detected releases, and scanner outbox;
* `postgres-notifier` for notification delivery state and notifier outbox.

The scanner keeps only the data it needs: `repo`, `last_seen_tag`, and `next_scan_at`.

The notification service stores delivery state and idempotency data, not subscriber source-of-truth data.

## Logging

Logs are not domain messages.

Services always write JSON logs to stdout and push logs to Vector by default. File duplication is disabled by default and enabled only when `LOG_FILE` is configured.

Kafka, RabbitMQ, and Logstash are not used for log transport.

## Consequences

### Positives

* Domains are separated by service module, database ownership, and Kafka contracts.
* Scanner and notification workloads can scale independently.
* Email delivery can retry without blocking HTTP subscription requests.
* Scanner does not depend on the subscription database schema.
* Notification service does not know how subscriptions are stored.
* Shared code is limited to infrastructure and contracts.
* Kafka and outbox provide durable service communication without direct service-to-service coupling.

### Negatives

* The system has more moving parts than the monolith.
* Kafka, outbox relays, and multiple databases must be operated locally and in CI.
* Event contracts must be versioned and tested.
* End-to-end debugging requires correlation ids across services.

## Status of legacy code

The old `cmd/api-server` and root `internal` tree have been removed from the target codebase.

The target architecture is under `services`, `pkg`, `api`, and `deployments`.
