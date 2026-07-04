## ADR-004: Export RED metrics to Prometheus and visualize them in Grafana

**Status:** Superseded for deployment details by ADR-007 and ADR-008  
**Date:** 2026-06-07  
**Author:** Oleksandr Makarov

## Context

> Historical note: this ADR selected Prometheus/Grafana and RED-style metrics for the original service shape. The target microservice topology keeps `/metrics` endpoints and uses new Prometheus/Grafana provisioning for the three services. The old RabbitMQ/Logstash observability overlay and `scripts/observability-smoke.*` scripts are no longer part of the codebase.

The service needs operational metrics that answer the basic production questions:

* How much traffic is the application receiving?
* How many requests fail?
* How long do requests take?
* Are background notification and scanner workers healthy?

The application already exposes HTTP endpoints and background workers, so RED metrics are a natural fit for the public API surface, with a few worker-specific counters for asynchronous processing.

## Decision

Use Prometheus as the metrics store and Grafana as the visualization layer.

The application owns metric definitions and exposes them on `/metrics`. Prometheus scrapes the application over Docker Compose networking. Grafana is provisioned with Prometheus as the default datasource and imports the Subber RED dashboard from version-controlled JSON.

## Implementation Details

* **Registry:** the application creates an explicit Prometheus registry in `main` and injects a `metrics.Metrics` instance into middleware and workers.
* **Runtime metrics:** Go runtime and process collectors are registered alongside application metrics.
* **HTTP RED metrics:**
  * `http_requests_total{method,route,status_code}` tracks request rate and status-class errors.
  * `http_request_duration_seconds{method,route}` tracks request latency via histogram buckets.
* **Worker metrics:**
  * `emails_sent_total`
  * `emails_failed_total`
  * `scan_cycles_total`
* **Logging pipeline metrics:**
  * `log_entries_enqueued_total`
  * `log_entries_dropped_total`
  * `log_entries_published_total`
  * `log_publish_errors_total`
* **Labels:** route templates are used instead of raw paths so labels stay bounded and tokens are never emitted as metric labels.
* **Target infrastructure:** all three services expose `/metrics`; Prometheus scrapes them through `deployments/docker/prometheus/prometheus.yml`.
* **Grafana provisioning:** Grafana loads the Prometheus datasource and `Subber Overview` dashboard from `deployments/docker/grafana`.
* **Logging metrics:** RabbitMQ log-queue metrics are obsolete because log transport moved to Vector in ADR-007.
* **Smoke verification:** runtime verification moved to `scripts/runtime-smoke.ps1` and `scripts/runtime-smoke.sh`.

## Consequences

### Positives

* RED metrics are queryable with PromQL and visualized without manual UI setup.
* Metrics are testable because the registry is injected instead of using the global default registry.
* Route labels are bounded and safe for aggregation.
* Prometheus target configuration is reproducible from source control.
* Grafana dashboard and datasource provisioning are reproducible from source control.

### Negatives

* Alerts still need to be recreated for the microservice topology.
* Histogram buckets use Prometheus defaults for now; they may need tuning after observing real latency distributions.
