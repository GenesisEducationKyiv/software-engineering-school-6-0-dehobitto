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

Static HTML is served locally, API calls are mocked in the browser — no backend needed. Everything runs inside Docker.

```sh
docker build -f tests/e2e/Dockerfile -t subber-e2e . && docker run --rm subber-e2e
```
