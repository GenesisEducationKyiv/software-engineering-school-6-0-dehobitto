go test -race ./...
go test -race -tags integration ./tests/integration/...
try {
    docker compose -f docker-compose.yml -f docker/docker-compose.e2e.yml up --build --abort-on-container-exit --exit-code-from e2e e2e
} finally {
    docker compose -f docker-compose.yml -f docker/docker-compose.e2e.yml down -v --remove-orphans
}
