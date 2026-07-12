#!/bin/sh
set -eu

COMPOSE_FILE="${COMPOSE_FILE:-compose.microservices.yml}"
TIMEOUT_SECONDS="${TIMEOUT_SECONDS:-90}"
START_STACK="${START_STACK:-false}"
BUILD="${BUILD:-false}"

step() {
  echo "==> $1"
}

wait_until() {
  name="$1"
  shift
  deadline=$(( $(date +%s) + TIMEOUT_SECONDS ))
  while :; do
    if "$@" >/tmp/subber-runtime-smoke.out 2>/tmp/subber-runtime-smoke.err; then
      echo "OK: $name"
      return 0
    fi
    if [ "$(date +%s)" -ge "$deadline" ]; then
      echo "Timed out waiting for $name" >&2
      cat /tmp/subber-runtime-smoke.err >&2 || true
      return 1
    fi
    sleep 2
  done
}

step "validating compose config"
docker compose -f "$COMPOSE_FILE" config --quiet

if [ "$START_STACK" = "true" ]; then
  step "starting compose stack"
  if [ "$BUILD" = "true" ]; then
    docker compose -f "$COMPOSE_FILE" up --build -d
  else
    docker compose -f "$COMPOSE_FILE" up -d
  fi
fi

step "checking service status"
docker compose -f "$COMPOSE_FILE" ps

check_url() {
  curl -fsS "$1" >/dev/null
}

check_elastic() {
  curl -fsS "http://localhost:9200/_cluster/health" | grep -Eq '"status":"(green|yellow)"'
}

check_kibana_ready() {
  curl -fsS "http://localhost:5601/api/status" | grep -Eq '"overall":\{"level":"(available|degraded)"'
}

check_kibana_dashboard() {
  curl -fsS "http://localhost:5601/api/saved_objects/dashboard/subber-main-dashboard" |
    grep -q '"title":"Main dashboard"'
}

check_prometheus_ready() {
  curl -fsS "http://localhost:9090/-/ready" >/dev/null
}

check_prometheus_targets() {
  for job in subscription-api scanner-service notification-service; do
    curl -fsS "http://localhost:9090/api/v1/query" \
      --get \
      --data-urlencode "query=up{job=\"$job\"}" |
      grep -q '"value":\[[^]]*,"1"\]'
  done
}

check_grafana_ready() {
  curl -fsS "http://localhost:3000/api/health" | grep -q '"database":"ok"'
}

check_grafana_datasource() {
  curl -fsS "http://localhost:3000/api/datasources/name/Prometheus" |
    grep -q '"type":"prometheus"'
}

check_grafana_dashboard() {
  curl -fsS "http://localhost:3000/api/dashboards/uid/subber-overview" |
    grep -q '"title":"Subber Overview"'
}

check_topics() {
  topics="$(docker compose -f "$COMPOSE_FILE" exec -T kafka /opt/kafka/bin/kafka-topics.sh --bootstrap-server localhost:9092 --list)"
  for topic in \
    subber.watchlist.events \
    subber.release.events \
    subber.notification.commands \
    subber.notification.retry.1m \
    subber.notification.retry.10m \
    subber.notification.dlq
  do
    echo "$topics" | grep -Fx "$topic" >/dev/null
  done
}

SMOKE_ID="runtime-smoke-$(date +%s)"
post_vector_log() {
  curl -fsS -X POST "http://localhost:8686" \
    -H "Content-Type: application/json" \
    --data "{\"time\":\"$(date -u +%Y-%m-%dT%H:%M:%SZ)\",\"level\":\"info\",\"msg\":\"runtime smoke log\",\"service\":\"runtime-smoke\",\"component\":\"script\",\"smoke_id\":\"$SMOKE_ID\"}" >/dev/null
}

check_vector_indexed() {
  curl -fsS -X POST "http://localhost:9200/subber-logs-*/_search" \
    -H "Content-Type: application/json" \
    --data "{\"size\":1,\"query\":{\"match_phrase\":{\"smoke_id\":\"$SMOKE_ID\"}}}" |
    grep -Eq '"value":[1-9]'
}

wait_until "subscription-api root" check_url "http://localhost:8080/"
wait_until "subscription-api metrics" check_url "http://localhost:8080/metrics"
wait_until "scanner-service metrics" check_url "http://localhost:8081/metrics"
wait_until "notification-service metrics" check_url "http://localhost:8082/metrics"
wait_until "mailpit api" check_url "http://localhost:8025/api/v1/messages"
wait_until "elasticsearch health" check_elastic
wait_until "kibana ready" check_kibana_ready
wait_until "kibana subber dashboard" check_kibana_dashboard
wait_until "prometheus ready" check_prometheus_ready
wait_until "prometheus service targets" check_prometheus_targets
wait_until "grafana ready" check_grafana_ready
wait_until "grafana prometheus datasource" check_grafana_datasource
wait_until "grafana subber dashboard" check_grafana_dashboard
wait_until "kafka topics" check_topics

step "checking vector to elasticsearch log path"
post_vector_log
wait_until "vector log indexed in elasticsearch" check_vector_indexed

echo "Runtime smoke OK"
