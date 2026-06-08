#!/bin/sh
set -eu

APP_URL="${APP_URL:-http://localhost:8080}"
PROMETHEUS_URL="${PROMETHEUS_URL:-http://localhost:9090}"
ELASTICSEARCH_URL="${ELASTICSEARCH_URL:-http://localhost:9200}"
GRAFANA_URL="${GRAFANA_URL:-http://localhost:3000}"
KIBANA_URL="${KIBANA_URL:-http://localhost:5601}"
TIMEOUT_SECONDS="${TIMEOUT_SECONDS:-90}"

wait_until() {
  name="$1"
  shift
  deadline=$(( $(date +%s) + TIMEOUT_SECONDS ))

  while :; do
    if "$@" >/tmp/subber-smoke-check.out 2>/tmp/subber-smoke-check.err; then
      echo "OK: $name"
      return 0
    fi
    if [ "$(date +%s)" -ge "$deadline" ]; then
      echo "Timed out waiting for $name" >&2
      cat /tmp/subber-smoke-check.err >&2 || true
      return 1
    fi
    sleep 3
  done
}

check_app_metrics() {
  curl -fsS "$APP_URL/metrics" | grep -q "http_requests_total"
}

check_prometheus_target() {
  curl -fsS "$PROMETHEUS_URL/api/v1/query?query=up%7Bjob%3D%22subber%22%7D" | grep -q '"value":\[[^]]*,"1"\]'
}

check_rabbitmq_target() {
  curl -fsS "$PROMETHEUS_URL/api/v1/query?query=up%7Bjob%3D%22rabbitmq%22%7D" | grep -q '"value":\[[^]]*,"1"\]'
}

check_prometheus_rules() {
  rules="$(curl -fsS "$PROMETHEUS_URL/api/v1/rules")"
  echo "$rules" | grep -q "SubberTargetDown"
  echo "$rules" | grep -q "SubberLogEntriesDropped"
  echo "$rules" | grep -q "SubberLogDeadLetterQueueNotEmpty"
}

check_grafana() {
  curl -fsS "$GRAFANA_URL/api/health" | grep -q '"database":"ok"'
}

check_kibana() {
  curl -fsS "$KIBANA_URL/api/status" | grep -q "available"
}

check_ilm() {
  curl -fsS "$ELASTICSEARCH_URL/_ilm/policy/subber-logs-7d" | grep -q "subber-logs-7d"
}

wait_until "application metrics endpoint" check_app_metrics
wait_until "Prometheus target" check_prometheus_target
wait_until "RabbitMQ Prometheus target" check_rabbitmq_target
wait_until "Prometheus alert rules" check_prometheus_rules
wait_until "Grafana health" check_grafana
wait_until "Kibana status" check_kibana
wait_until "Elasticsearch ILM policy" check_ilm

REQUEST_ID="smoke-$(date +%s)"
TOKEN="00000000-0000-0000-0000-000000000000"
STATUS="$(curl -sS -o /tmp/subber-smoke-response.out -w "%{http_code}" -H "X-Request-ID: $REQUEST_ID" "$APP_URL/api/confirm/$TOKEN")"
if [ "$STATUS" != "404" ]; then
  echo "expected app request to return 404, got $STATUS" >&2
  exit 1
fi

check_request_metric() {
  curl -fsS "$PROMETHEUS_URL/api/v1/query" \
    --get \
    --data-urlencode 'query=http_requests_total{route="/api/confirm/:token",status_code="404"}' |
    grep -q '"result":\[{'
}

check_request_log() {
  curl -fsS -X POST "$ELASTICSEARCH_URL/subber-logs-*/_search" \
    -H "Content-Type: application/json" \
    --data "{\"size\":1,\"query\":{\"bool\":{\"must\":[{\"match_phrase\":{\"request_id\":\"$REQUEST_ID\"}},{\"match_phrase\":{\"component\":\"http\"}}]}}}" |
    grep -q '"value":[1-9]'
}

wait_until "request metric in Prometheus" check_request_metric
wait_until "request log in Elasticsearch" check_request_log

echo "Observability smoke test passed for request_id=$REQUEST_ID"
