# Testing

Three test suites. Only prerequisites: **git**, **docker**, **Go**.

---

## Unit Tests

No external dependencies. Run anywhere.

```sh
go test ./...
```

---

## Integration Tests

Spin up PostgreSQL automatically via [testcontainers-go](https://golang.testcontainers.org/). Docker must be running.

```sh
go test -v -tags integration ./tests/integration/...
```

---

## E2E Tests (Playwright)

No backend needed — API calls are mocked inside the browser. Serves the static HTML with `npx serve`.

Docker must be running. Build context is the repo root.

```sh
docker build -f tests/e2e/Dockerfile -t subber-e2e .
docker run --rm subber-e2e
```

To run with a custom static dir:

```sh
docker run --rm -e STATIC_DIR=/app/e2e/static subber-e2e
```

---

## CI

| Workflow | Trigger | Command |
|---|---|---|
| `go-unittest.yml` | push/PR → main | `go test -v ./...` |
| `test-integration.yml` | push/PR → main | `go test -v -tags integration ./tests/integration/...` |
| `test-e2e.yml` | push/PR → main | `docker build` + `docker run` |
