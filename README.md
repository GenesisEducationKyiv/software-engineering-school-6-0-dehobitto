# Subber

Subber watches GitHub repositories for new releases and sends email notifications to subscribers.

The system is split into three Go services connected through Kafka:

* `subscription-api` owns the HTTP API, subscriptions, confirmation/unsubscribe flow, and release fan-out.
* `scanner-service` owns repository watch state, GitHub release polling, and Redis-backed GitHub cache.
* `notification-service` owns email delivery, idempotency, retries, and DLQ handling.

Shared infrastructure code lives in `pkg`. Service-owned code stays under `services/*`.
Notification commands can run through either the default Kafka/outbox transport or the direct gRPC transport documented in [gRPC notification transport](docs/grpc-notification-transport.md).

## Requirements

* Go `1.26.2`
* Docker with Docker Compose `2.20.3` or newer
* PowerShell or POSIX shell for helper scripts

## Structure

| Path | Purpose |
| --- | --- |
| `services/subscription-api` | HTTP API and subscription domain |
| `services/scanner-service` | GitHub scanner domain |
| `services/notification-service` | email delivery domain |
| `pkg` | shared Go packages |
| `deployments/docker` | local infrastructure compose, Prometheus, Grafana, Vector, Kibana |
| `api/openapi` | HTTP contract |
| `api/asyncapi` | Kafka event contract |
| `docs` | development, testing, observability, logging, architecture notes |
| `scripts` | local smoke and E2E helpers |
| `tests/e2e` | Playwright E2E stack |
| `tests/integration` | cross-package integration tests |

## Run

```sh
docker compose -f compose.microservices.yml up --build -d
```

Stop:

```sh
docker compose -f compose.microservices.yml down
```

Run full E2E from a clean Docker stack:

```sh
sh scripts/e2e.sh
```

or:

```powershell
powershell -ExecutionPolicy Bypass -File scripts/e2e.ps1
```

## Local Endpoints

| Service | URL |
| --- | --- |
| Subscription API | http://localhost:8080 |
| Subscription API metrics | http://localhost:8080/metrics |
| Scanner metrics | http://localhost:8081/metrics |
| Notification metrics | http://localhost:8082/metrics |
| Notification gRPC | localhost:9093 |
| Kafka | localhost:9092 |
| Kafka exporter | http://localhost:9308/metrics |
| Redis | localhost:6379 |
| Prometheus | http://localhost:9090 |
| Grafana | http://localhost:3000 |
| Mailpit | http://localhost:8025 |
| Elasticsearch | http://localhost:9200 |
| Kibana | http://localhost:5601 |
| Vector | http://localhost:8686 |

## Notification Transport Comparison

Local A/B load comparison can be run with:

```powershell
powershell -ExecutionPolicy Bypass -File scripts/compare-notification-transports.ps1 -Build -Requests 200 -Concurrency 20
```

Latest local result:

| Metric | Result |
| --- | --- |
| gRPC - Kafka RPS delta | `+91.55 req/s` |
| gRPC - Kafka p95 latency delta | `-562.88 ms` |

The delta is calculated as `grpc_value - kafka_value`. In this local run, gRPC handled more requests per second and had lower p95 latency because it skips outbox persistence, relay polling, Kafka broker round trips, and consumer dispatch for the initial notification command.

Kafka/outbox remains the more durable option: it stores the notification intent in the database and can deliver it later if `notification-service` is temporarily unavailable. gRPC is faster in this benchmark, but it depends on `notification-service` being available during the request.

## Docs

* [Development](docs/development.md)
* [Testing](docs/testing.md)
* [Observability](docs/observability.md)
* [Logging](docs/logging.md)
* [gRPC notification transport](docs/grpc-notification-transport.md)
* [Architecture delta](docs/SDD-changelog.md)
* [Original SDD](docs/SDD.md)

## Contracts

* [OpenAPI](api/openapi/subscription-api.yaml)
* [AsyncAPI](api/asyncapi/subber-events.yaml)
* [Protobuf](api/proto/notification/v1/notification.proto)
