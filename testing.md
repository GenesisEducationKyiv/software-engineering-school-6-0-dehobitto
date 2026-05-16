# Testing

## Unit Tests

No external dependencies.

```sh
go test ./...
```

## Integration Tests

PostgreSQL підіймається автоматично через Docker (testcontainers). Docker має бути запущений.

```sh
go test -tags integration ./tests/integration/...
```

## E2E Tests

Статичний HTML роздається локально, API-запити мокаються в браузері — бекенд не потрібен. Все запускається в Docker.

```sh
docker build -f tests/e2e/Dockerfile -t subber-e2e . && docker run --rm subber-e2e
```
