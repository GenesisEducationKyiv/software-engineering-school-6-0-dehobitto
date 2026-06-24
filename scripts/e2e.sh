#!/usr/bin/env sh
set -eu

COMPOSE_FILES="-f compose.microservices.yml -f tests/e2e/compose.yml"

docker compose $COMPOSE_FILES down -v --remove-orphans

cleanup() {
  docker compose $COMPOSE_FILES down -v --remove-orphans
}
trap cleanup EXIT

docker compose $COMPOSE_FILES build --provenance=false --sbom=false
docker compose $COMPOSE_FILES up --no-build -d \
  wiremock \
  kafka \
  kafka-topics-init \
  kafka-exporter \
  redis \
  postgres-api \
  postgres-scanner \
  postgres-notifier \
  subscription-api-migrate \
  scanner-service-migrate \
  notification-service-migrate \
  subscription-api \
  scanner-service \
  notification-service \
  subscription-api-outbox-relay \
  scanner-service-outbox-relay \
  notification-service-outbox-relay \
  mailpit \
  elasticsearch \
  kibana \
  kibana-import \
  vector \
  prometheus \
  grafana
docker compose $COMPOSE_FILES run --rm e2e
