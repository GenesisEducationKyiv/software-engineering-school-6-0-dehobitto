# Testing

Prerequisites: **git**, **docker**, **Go**.

## Run All Tests

**Linux / macOS / Git Bash:**
```sh
sh scripts/test.sh
```

**Windows (PowerShell):**
```sh
.\scripts\test.ps1
```

## Manual test + k6 loadtest

```bash
# start app and logging stack first
docker compose -f docker-compose.yml -f docker/docker-compose.logging.yml -f docker/docker-compose.observability.yml up --build -d

# then run k6 (exits automatically when done)
docker compose -f docker-compose.yml -f docker/docker-compose.loadtest.yml run --rm k6
```

---

## Unit Tests

No external dependencies.

```sh
go test ./...
```

## Integration Tests

PostgreSQL spins up automatically via Docker (testcontainers). Docker must be running.

```sh
go test -tags integration ./tests/integration/...
```

## E2E Tests

The E2E suite launches a complete test environment: frontend, real backend, PostgreSQL, Redis, and a fake GitHub API. API calls are not mocked in the browser.

```sh
docker compose -f docker-compose.yml -f docker/docker-compose.e2e.yml up --build --abort-on-container-exit --exit-code-from e2e e2e
docker compose -f docker-compose.yml -f docker/docker-compose.e2e.yml down -v
```


## Observability Smoke Test

Requires the full Docker Compose observability stack to be running:

```sh
docker compose -f docker-compose.yml -f docker/docker-compose.logging.yml -f docker/docker-compose.observability.yml up --build -d
```

Then run:

```sh
# Linux / macOS / Git Bash
sh scripts/observability-smoke.sh

# Windows PowerShell
.\scripts\observability-smoke.ps1
```

The smoke test checks app metrics, Prometheus target/rules, Grafana health, Kibana health, Elasticsearch ILM setup, one request metric in Prometheus, and the matching request log in Elasticsearch.
