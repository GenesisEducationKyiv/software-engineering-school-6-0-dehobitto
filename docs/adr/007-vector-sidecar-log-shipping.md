## ADR-007: Use Vector sidecar for best-effort log shipping

**Status:** Accepted

**Date:** 2026-06-10

**Author:** Oleksandr Makarov

> Supersedes ADR-003 for log transport.

## Context

* Subber is being split into multiple services, and every service must keep structured logs available through container stdout.
* Logs are observability data, not business workflow messages. Losing some logs during local failures is acceptable.
* Domain events use Kafka, but log transport should not share the same broker because logs have different reliability and retention needs.
* The previous RabbitMQ/Logstash logging path is too heavy for the target microservice topology.

## Variants considered

**1. Brokered log delivery with RabbitMQ or Kafka**

* **Positives:** durable buffering and consumer acknowledgements.
* **Negatives:** treats logs like business-critical messages; adds broker traffic, operational coupling, and retry/DLQ complexity that logs do not need.

**2. File-only logging**

* **Positives:** simple and easy to inspect locally.
* **Negatives:** does not provide a standard path to Elasticsearch without an additional collector.

**3. Best-effort push to Vector sidecar**

* **Positives:** services keep stdout logging, sidecar push is enabled by default but can be disabled by config, Vector buffers logs locally and ships batches to Elasticsearch.
* **Negatives:** log delivery is best-effort; sidecar outages may drop logs if the service-side buffer overflows.

## Final choice

**Vector sidecar selected.**

Services always write structured JSON logs to stdout. By default, services also push logs asynchronously to a local Vector sidecar. Vector buffers logs and sends batches to Elasticsearch. Kafka, RabbitMQ, and Logstash are not used for log transport in the target architecture.

## Implementation Details

* **Default behavior:** stdout logging plus Vector sidecar push.
* **Sidecar push:** enabled by default with `LOG_SIDECAR_ENABLED=true`; disabled explicitly with `LOG_SIDECAR_ENABLED=false`.
* **Sidecar URL:** configured with `LOG_SIDECAR_URL`, defaulting to `http://vector:8686`.
* **File duplication:** disabled by default with `LOG_FILE=`; enabled only when a file path is configured.
* **Delivery semantics:** best-effort. Logging must not block business request handling indefinitely.
* **Vector input:** HTTP server input receives JSON log entries from services.
* **Vector output:** Elasticsearch output writes batched logs to `subber-logs-*`.
* **Batching:** Vector flushes Elasticsearch batches when they reach `524288` bytes, or after `5s` under low log volume.
* **Legacy path:** RabbitMQ/Logstash log transport remains historical and is not used by new microservice deployments.

## Consequences

### Positives

* Kafka remains dedicated to domain events and jobs.
* Services can still run with only stdout logs when `LOG_SIDECAR_ENABLED=false`.
* Elasticsearch log shipping is on by default in the target compose topology.
* Vector is lighter than the previous RabbitMQ plus Logstash pipeline.

### Negatives

* Log delivery is not guaranteed.
* Service-side async buffering and Vector sidecar health need monitoring in environments where Elasticsearch logs matter.
