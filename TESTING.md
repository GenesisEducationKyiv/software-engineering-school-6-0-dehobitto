# Testing

Prerequisites: **Go**, **Docker**, and Docker Compose `2.20.3` or newer.

## Unit Tests

```sh
go test ./pkg/... ./services/subscription-api/... ./services/scanner-service/... ./services/notification-service/...
```

## Integration Tests

PostgreSQL spins up automatically through testcontainers. Docker must be running.

```sh
go test -tags integration ./tests/integration/... ./services/subscription-api/...
```

## Compose Validation

```sh
docker compose -f compose.microservices.yml config --quiet
```

## Runtime Smoke

Runtime smoke validates that the local stack is alive: service endpoints, metrics endpoints, Prometheus targets, Grafana provisioning, Kafka topics, Mailpit, Elasticsearch, and Vector log indexing.

```powershell
powershell -ExecutionPolicy Bypass -File scripts/runtime-smoke.ps1
```

or:

```sh
sh scripts/runtime-smoke.sh
```

Start the stack from the smoke script when needed:

```powershell
powershell -ExecutionPolicy Bypass -File scripts/runtime-smoke.ps1 -StartStack -Build
```

or:

```sh
START_STACK=true BUILD=true sh scripts/runtime-smoke.sh
```

## Kafka E2E

Kafka E2E verifies the business flow through the message bus:

```text
subscribe
-> confirm
-> RepoWatchStartRequested
-> scanner watchlist
-> ReleaseDetected
-> NotificationSendRequested
-> notification-service sent delivery
```

```powershell
powershell -ExecutionPolicy Bypass -File scripts/kafka-e2e.ps1
```

or:

```sh
sh scripts/kafka-e2e.sh
```

## Load Test

The load test uses `k6` and targets the running subscription API. If `k6` is not installed locally, run it through Docker.

```powershell
docker run --rm -i `
  -e BASE_URL=http://host.docker.internal:8080 `
  -e API_KEY=dev-api-key `
  -v ${PWD}/scripts:/scripts `
  grafana/k6 run /scripts/loadtest.js
```

Use a different target URL or API key when needed by changing `BASE_URL` and `API_KEY`.

```sh
docker run --rm -i \
  -e BASE_URL=http://host.docker.internal:8080 \
  -e API_KEY=dev-api-key \
  -v "$PWD/scripts:/scripts" \
  grafana/k6 run /scripts/loadtest.js
```
