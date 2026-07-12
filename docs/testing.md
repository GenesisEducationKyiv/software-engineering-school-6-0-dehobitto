# Testing

Prerequisites: Go, Docker, and Docker Compose `2.20.3` or newer.

## Unit Tests

```sh
go test ./pkg/... ./services/subscription-api/... ./services/scanner-service/... ./services/notification-service/...
```

## Architecture Tests

Architecture tests verify service boundaries and layer dependency rules without Docker.

```sh
go test ./tests/architecture
```

## Integration Tests

PostgreSQL starts automatically through testcontainers. Docker must be running.

```sh
go test -tags integration ./tests/integration/... ./services/subscription-api/internal/integration/...
```

## Full E2E

The full E2E script starts from a clean Docker Compose stack, builds images, runs migrations, starts services, and runs Playwright.

```sh
sh scripts/e2e.sh
```

or:

```powershell
powershell -ExecutionPolicy Bypass -File scripts/e2e.ps1
```

## Runtime Smoke

Runtime smoke validates compose config, HTTP endpoints, metrics endpoints, Prometheus targets, Grafana provisioning, Kafka topics, Mailpit, Elasticsearch, Kibana, and Vector log indexing.

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

Kafka E2E verifies the asynchronous business flow:

```text
subscribe
-> confirm
-> RepoWatchSagaRequested
-> StartWatchingRepo
-> RepoWatchStarted
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

The load test uses `k6` against a running subscription API.

```powershell
docker run --rm -i `
  -e BASE_URL=http://host.docker.internal:8080 `
  -e API_KEY= `
  -v ${PWD}/scripts:/scripts `
  grafana/k6 run /scripts/loadtest.js
```

```sh
docker run --rm -i \
  -e BASE_URL=http://host.docker.internal:8080 \
  -e API_KEY= \
  -v "$PWD/scripts:/scripts" \
  grafana/k6 run /scripts/loadtest.js
```
