## ADR-004: Export RED metrics to Prometheus and visualize them in Grafana

**Status:** Accepted  
**Date:** 2026-06-07  
**Author:** Oleksandr Makarov

## Context

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
* **Infrastructure:** `docker/docker-compose.observability.yml` adds Prometheus and Grafana. Prometheus config lives in `docker/prometheus/prometheus.yml`. Grafana datasource and dashboard provisioning live under `docker/grafana/provisioning`.
* **RabbitMQ metrics:** the logging overlay enables the RabbitMQ Prometheus plugin. Prometheus scrapes RabbitMQ to monitor the main log queue and dead-letter queue.
* **Alerts:** Prometheus alert rules are provisioned from `docker/prometheus/rules/subber-alerts.yml` for target availability, 5xx ratio, p95 latency, email failures, dropped log entries, DLQ messages, and log queue backlog.
* **Smoke verification:** `scripts/observability-smoke.ps1` and `scripts/observability-smoke.sh` verify the running local stack end to end.

## Consequences

### Positives

* RED metrics are queryable with PromQL and visualized without manual UI setup.
* Metrics are testable because the registry is injected instead of using the global default registry.
* Route labels are bounded and safe for aggregation.
* Grafana dashboards and datasources are reproducible from source control.
* Alert rules are version-controlled with the rest of the observability stack.
* DLQ and backlog conditions are visible from metrics instead of requiring manual RabbitMQ inspection.

### Negatives

* Additional services increase local compose resource usage.
* Histogram buckets use Prometheus defaults for now; they may need tuning after observing real latency distributions.
* The first dashboard is intentionally focused on core RED and worker health; deeper business metrics can be added once operational baselines are known.
* Alert rules are loaded locally but no Alertmanager or external notification channel is configured yet.
