# Observability

Subber uses three observability paths:

* structured JSON logs;
* Prometheus metrics;
* local smoke checks for service readiness and end-to-end flows.

## Target Stack

| Layer | Tool | Local URL |
| --- | --- | --- |
| Subscription API | HTTP | http://localhost:8080 |
| Subscription API metrics | `/metrics` | http://localhost:8080/metrics |
| Scanner metrics | `/metrics` | http://localhost:8081/metrics |
| Notification metrics | `/metrics` | http://localhost:8082/metrics |
| Kafka | Domain events and jobs | localhost:9092 |
| Redis | Scanner GitHub API cache | localhost:6379 |
| Log search | Elasticsearch | http://localhost:9200 |
| Log dashboard/search UI | Kibana | http://localhost:5601 |
| Log collector | Vector | http://localhost:8686 |
| Metrics scraper | Prometheus | http://localhost:9090 |
| Metrics dashboards | Grafana | http://localhost:3000 |

Prometheus scrapes all three services. Grafana is provisioned with a Prometheus datasource and the `Subber Overview` dashboard.

Kibana is included for log inspection against Elasticsearch. The restored dashboard artifact is stored at [deployments/docker/kibana/dashboards.ndjson](../deployments/docker/kibana/dashboards.ndjson) and is mounted into the Kibana container at `/usr/share/kibana/dashboards/subber-dashboard.ndjson`.

## Run

The target microservice topology is started from the root compose file:

```sh
docker compose -f compose.microservices.yml up --build -d
```

This root file uses Docker Compose `include`, so Docker Compose `2.20.3` or newer is required.

Compose service configuration is env-based. Common local defaults live in [deployments/docker/env/common.env.example](../deployments/docker/env/common.env.example), and service-specific defaults live in each service directory.

Stop the stack:

```sh
docker compose -f compose.microservices.yml down
```

## Logs

Services always write JSON logs to stdout. Vector push is enabled by default:

```text
LOG_SIDECAR_ENABLED=true
LOG_SIDECAR_URL=http://vector:8686
LOG_FILE=
```

`LOG_FILE` is empty by default. Set it only when a service must also duplicate logs to a local file.

Vector batches accepted logs to Elasticsearch when the batch reaches `524288` bytes, or after `5s` if traffic is low. Kafka, RabbitMQ, and Logstash are not used for logging in the target architecture.

More details: [Logging](logging.md).

## Metrics

Current status:

* `subscription-api` exposes `/metrics`;
* `scanner-service` exposes `/metrics`;
* `notification-service` exposes `/metrics`;
* Prometheus scrape config lives in [deployments/docker/prometheus/prometheus.yml](../deployments/docker/prometheus/prometheus.yml).
* Grafana provisioning lives in [deployments/docker/grafana/provisioning](../deployments/docker/grafana/provisioning).

Expected metric groups:

* HTTP RED metrics for `subscription-api`;
* scanner claim/scan/release counters;
* notification sent/failed/retried/dead counters;
* Kafka consumer lag and retry topic backlog, if exposed by infrastructure.

## Smoke Checks

Runtime smoke should verify:

* service containers start;
* each service connects to its own PostgreSQL database;
* services connect to Kafka;
* Redis is available for scanner cache;
* migrations run;
* Vector starts and accepts logs by default.
* Kibana is ready for Elasticsearch log inspection.
* Prometheus is ready and sees all three service targets as `up`.
* Grafana is ready, has the Prometheus datasource, and loads the `Subber Overview` dashboard.

End-to-end smoke should verify:

```text
subscribe
-> confirmation NotificationSendRequested
-> confirm
-> RepoWatchStartRequested
-> scanner watchlist
-> ReleaseDetected
-> NotificationSendRequested
-> notification-service delivery
```

## Access Model

The local Docker Compose stack publishes ports for development inspection. This is not a production access model. In production, keep Kafka, Redis, PostgreSQL, Elasticsearch, Vector, Prometheus, and internal metrics endpoints on private networks.
