# Observability

This project uses logs, metrics, dashboards, alerts, and smoke checks as one local observability workflow.

## Stack

| Layer | Tool | Local URL |
| --- | --- | --- |
| Application metrics | `/metrics` | http://localhost:8080/metrics |
| Metrics store | Prometheus | http://localhost:9090 |
| Metrics dashboards | Grafana | http://localhost:3000 |
| Log broker | RabbitMQ | http://localhost:15672 |
| Log search | Elasticsearch | http://localhost:9200 |
| Log dashboards | Kibana | http://localhost:5601 |

Grafana login for local development is `admin / admin`. RabbitMQ local login is `guest / guest`.

## Run

Start the complete local observability stack:

```sh
docker compose -f docker-compose.yml -f docker/docker-compose.logging.yml -f docker/docker-compose.observability.yml up --build -d
```

Generate traffic:

```sh
docker compose -f docker-compose.yml -f docker/docker-compose.loadtest.yml run --rm k6
```

Run the smoke check:

```sh
# Linux / macOS / Git Bash
sh scripts/observability-smoke.sh

# Windows PowerShell
.\scripts\observability-smoke.ps1
```

The smoke check verifies:

- application `/metrics`;
- Prometheus `subber` target;
- Prometheus `rabbitmq` target;
- Prometheus alert rules;
- Grafana health;
- Kibana health;
- Elasticsearch 7-day ILM policy;
- one HTTP request metric in Prometheus;
- the matching request log in Elasticsearch.

## Metrics

Main RED metrics:

- `http_requests_total{method,route,status_code}`;
- `http_request_duration_seconds{method,route}`;

Worker metrics:

- `emails_sent_total`;
- `emails_failed_total`;
- `scan_cycles_total`;

Logging pipeline metrics:

- `log_entries_enqueued_total`;
- `log_entries_dropped_total`;
- `log_entries_published_total`;
- `log_publish_errors_total`;

RabbitMQ metrics are scraped from the `rabbitmq` Prometheus endpoint and used to monitor queue backlog and DLQ state.

## Alerts

Rules live in `docker/prometheus/rules/subber-alerts.yml`.

Current local rules:

- `SubberTargetDown`;
- `SubberHighHTTP5xxRatio`;
- `SubberHighHTTPP95Latency`;
- `SubberEmailSendFailures`;
- `SubberLogEntriesDropped`;
- `SubberLogDeadLetterQueueNotEmpty`;
- `SubberLogQueueBacklog`.

These rules are loaded by Prometheus. Alertmanager and external notification channels are intentionally not configured in this local stack.

## Dashboards

Grafana provisioning:

- datasource: `docker/grafana/provisioning/datasources/prometheus.yml`;
- dashboard provider: `docker/grafana/provisioning/dashboards/dashboards.yml`;
- dashboard JSON: `docker/grafana/dashboards/subber-red.json`.

Kibana dashboard import:

- saved object export: `docker/kibana/dashboards.ndjson`;
- setup service: `kibana-setup` in `docker/docker-compose.logging.yml`.

## Access Model

The local Docker Compose stack publishes observability ports so the stack is easy to inspect during development. This is not a production access model.

For production, keep Prometheus, Elasticsearch, RabbitMQ management, and internal metrics endpoints on private networks. Put Grafana and Kibana behind authentication, SSO, VPN, or a protected reverse proxy.
