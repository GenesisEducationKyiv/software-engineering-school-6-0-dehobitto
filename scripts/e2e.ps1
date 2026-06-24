$ErrorActionPreference = "Stop"

$composeFiles = @(
  "-f", "compose.microservices.yml",
  "-f", "tests/e2e/compose.yml"
)

docker compose @composeFiles down -v --remove-orphans

try {
  docker compose @composeFiles build --provenance=false --sbom=false
  docker compose @composeFiles up --no-build -d `
    wiremock `
    kafka `
    kafka-topics-init `
    kafka-exporter `
    redis `
    postgres-api `
    postgres-scanner `
    postgres-notifier `
    subscription-api-migrate `
    scanner-service-migrate `
    notification-service-migrate `
    subscription-api `
    scanner-service `
    notification-service `
    subscription-api-outbox-relay `
    scanner-service-outbox-relay `
    notification-service-outbox-relay `
    mailpit `
    elasticsearch `
    kibana `
    kibana-import `
    vector `
    prometheus `
    grafana
  docker compose @composeFiles run --rm e2e
}
finally {
  docker compose @composeFiles down -v --remove-orphans
}
