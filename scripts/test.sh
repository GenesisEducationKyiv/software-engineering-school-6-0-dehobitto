#!/bin/sh
set -e

go test ./...
go test -tags integration ./tests/integration/...
docker build -f tests/e2e/Dockerfile -t subber-e2e .
docker run --rm subber-e2e
