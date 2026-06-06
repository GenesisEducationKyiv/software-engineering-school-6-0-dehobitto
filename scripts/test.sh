#!/bin/sh
set -e

go test -race ./...
go test -race -tags integration ./tests/integration/...
docker build -f tests/e2e/Dockerfile -t subber-e2e .
docker run --rm subber-e2e
