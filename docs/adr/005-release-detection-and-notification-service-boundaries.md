## ADR-005: Split release detection and notification delivery into separate services

**Status:** Accepted

**Date:** 2026-06-10

**Author:** Oleksandr Makarov

## Context

* Subber currently runs HTTP API, GitHub scanning, and email delivery in one application process.
* The scanner checks GitHub releases and currently creates in-memory notification jobs through a Go channel.
* Email delivery is slower than release scanning and needs retries, durable jobs, and independent scaling.
* After extracting services, the subscription database should remain owned by the subscription API instead of being shared freely between services.
* Scanner workers must be horizontally scalable without scanning the same repositories in every instance.

## Variants considered

**1. Scanner reads the subscription database directly**

* **Positives:** simplest migration from the current code.
* **Negatives:** scanner becomes coupled to the subscription schema and knows too much about subscribers.

**2. Scanner keeps its own repository watchlist**

* **Positives:** scanner no longer needs direct access to the subscription database.
* **Negatives:** requires extra synchronization events and still leaves open the question of who expands a release into subscriber emails.

**3. Scanner only publishes release events**

* **Positives:** clear responsibilities: scanner detects releases, subscription API owns subscribers, notifier sends emails.
* **Negatives:** requires Kafka-based communication, durable message handling, and explicit idempotency.

## Final choice

**Option 3 selected.**

Subber will be split into three services:

* `subscription-api` owns HTTP routes, subscription lifecycle, subscriber data, and expansion of release events into notification commands.
* `scanner-service` owns GitHub polling, GitHub API integration, release comparison, scanner read model, and GitHub response cache.
* `notification-service` owns SMTP delivery, retry handling, delivery state, and horizontal scaling of email workers.

The scanner service will not read subscriber emails or create email jobs directly. It will detect new GitHub releases and publish a `ReleaseDetected` event. The subscription API will consume that event, find matching subscribers in its own database, and publish durable `NotificationSendRequested` commands. The notification service will consume those commands and send emails with at-least-once delivery semantics.

## Implementation Details

* **Repository layout:** services live under `services/`; shared Go code lives under `pkg/`; HTTP and Kafka contracts live under `api/`; deployment manifests live under `deployments/`.
* **Go workspace:** the repository uses `go.work` with separate modules for `services/subscription-api`, `services/scanner-service`, `services/notification-service`, and `pkg`.
* **API contracts:** OpenAPI/Swagger contracts are stored under `api/openapi/`; Kafka contracts are documented with AsyncAPI under `api/asyncapi/`.
* **Databases:** each service owns its own PostgreSQL database/container: `postgres-api`, `postgres-scanner`, and `postgres-notifier`.
* **Scanner read model:** scanner stores `repo`, `last_seen_tag`, and `next_scan_at`.
* **Scanner batching:** scanner instances fetch due repositories in configurable batches using `FOR UPDATE SKIP LOCKED` so multiple instances do not process the same rows.
* **Scanner cache:** Redis remains a scanner dependency for GitHub API response caching. The scanner uses a cache proxy/decorator around the GitHub release provider, so scanner logic calls one interface while cache lookup and GitHub fallback stay hidden behind it.
* **Watchlist events:** `subscription-api` emits `RepoWatchStartRequested` when active subscriptions for a repo move from `0` to `1`, and `RepoWatchStopRequested` when they move from `1` to `0`. The scanner deletes the repo from its read model on stop instead of keeping disabled rows.
* **Release events:** `scanner-service` emits `ReleaseDetected` when it observes a new tag for a watched repository.
* **Notification commands:** `subscription-api` emits `NotificationSendRequested` per subscriber after consuming `ReleaseDetected`.
* **Notification identity:** notification commands include a `notification_id` UUID and `idempotency_key = repo:tag:email_hash`; `notification-service` enforces uniqueness in its database.
* **Notification delivery:** email delivery is at-least-once. Retries are limited by config and failed final attempts go to DLQ.
* **Outbox:** services use transactional outbox tables. A shared relay implementation is deployed once per service database to publish outbox rows to Kafka.
* **Logs and metrics:** each service emits structured logs to stdout and exposes its own metrics. Log shipping is handled separately by ADR-007.

## Consequences

### Positives

* Scanner does not depend on the subscription database schema.
* Notification delivery can be retried and scaled independently.
* In-memory notification jobs are replaced with durable broker messages.
* Scanner and notifier instances can be scaled independently.
* Service ownership boundaries are database-level and code-level, not only package-level.
* Service responsibilities are easy to explain and test separately.

### Negatives

* More services must be built, deployed, and monitored.
* Kafka failures and duplicate messages must be handled explicitly.
* The subscription API needs a background consumer for release events.
* Transactional outbox and relay processes add operational moving parts.
