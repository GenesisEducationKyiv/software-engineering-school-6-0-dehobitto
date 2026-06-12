## ADR-006: Use Kafka for domain events

**Status:** Accepted

**Date:** 2026-06-10

**Author:** Oleksandr Makarov

## Context

* The modular architecture splits Subber into `subscription-api`, `scanner-service`, and `notification-service`.
* These services must communicate without direct runtime dependencies on each other.
* Notification delivery, release detection, and watchlist synchronization must survive service restarts.
* Domain events and jobs are business workflow messages. They need durable buffering, consumer offsets, replay, and consumer groups.
* Logs are observability data and are handled separately by ADR-007.

## Variants considered

**1. Use direct service-to-service HTTP calls**

* **Positives:** simple request/response model and no broker dependency.
* **Negatives:** introduces runtime coupling between services; notifier or scanner outages would directly affect upstream request paths.

**2. Use RabbitMQ for domain jobs**

* **Positives:** good fit for work queues and acknowledgements.
* **Negatives:** less convenient for event replay and retained event streams; keeps the architecture centered around queues instead of domain events.

**3. Use Kafka for domain events and jobs**

* **Positives:** durable append-only topics, buffering, consumer offsets, replay, consumer groups, and partition-based scaling.
* **Negatives:** retry and DLQ flows must be designed explicitly with retry topics.

## Final choice

**Kafka selected for domain events and jobs.**

Services communicate through Kafka topics and do not call each other directly for asynchronous workflows. Kafka is not used for log transport.

## Implementation Details

* **Message envelope:** every domain message contains `event_id`, `event_type`, `occurred_at`, `source`, `correlation_id`, and `payload`.
* **Event contracts:** Kafka contracts are documented with AsyncAPI under `api/asyncapi/`.
* **Topics:**
  * `subber.watchlist.events` - `RepoWatchStartRequested`, `RepoWatchStopRequested`.
  * `subber.release.events` - `ReleaseDetected`.
  * `subber.notification.commands` - `NotificationSendRequested`.
  * `subber.notification.retry.1m` and `subber.notification.retry.10m` - delayed notification retries.
  * `subber.notification.dlq` - notifications that exhausted retry attempts.
* **Kafka keys:**
  * Watchlist events use `repo`.
  * Release events use `repo`.
  * Notification commands use `email_hash`.
* **Transactional outbox:** services write outgoing domain messages into their own outbox table in the same database transaction as the business change.
* **Outbox relay:** a shared relay implementation publishes outbox rows to Kafka, deployed separately per service database.
* **Idempotency:** consumers use database uniqueness constraints and `ON CONFLICT DO NOTHING` for duplicate-safe processing.
* **Notification retries:** `notification-service` consumes main and retry topics itself. Retry attempts and delays are configured through service config.

## Consequences

### Positives

* Services depend on Kafka contracts instead of direct knowledge of each other.
* Consumer groups allow scanner and notifier workloads to scale by partitions.
* Kafka offsets and retained topics support replay and delayed recovery after service outages.

### Negatives

* Kafka becomes a critical infrastructure dependency.
* Retry and DLQ behavior must be implemented explicitly with retry topics.
* Transactional outbox and relay processes are required to avoid losing events around database commits.
* Deployment becomes heavier than a direct HTTP-only service topology.
