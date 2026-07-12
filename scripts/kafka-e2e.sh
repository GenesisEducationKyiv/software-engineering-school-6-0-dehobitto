#!/bin/sh
set -eu

COMPOSE_FILE="${COMPOSE_FILE:-compose.microservices.yml}"
BASE_URL="${BASE_URL:-http://localhost:8080}"
REPO="${REPO:-cli/cli}"
API_KEY="${API_KEY:-dev-api-key}"
TIMEOUT_SECONDS="${TIMEOUT_SECONDS:-120}"

step() {
  echo "==> $1"
}

first_non_empty_line() {
  awk 'NF { print; exit }'
}

wait_value() {
  name="$1"
  command="$2"
  predicate="$3"
  deadline=$(( $(date +%s) + TIMEOUT_SECONDS ))
  while :; do
    value="$(sh -c "$command" | first_non_empty_line || true)"
    if value="$value" sh -c "$predicate" >/dev/null 2>&1; then
      echo "$value"
      echo "OK: $name" >&2
      return 0
    fi
    if [ "$(date +%s)" -ge "$deadline" ]; then
      echo "Timed out waiting for $name. Last value: $value" >&2
      return 1
    fi
    sleep 2
  done
}

SUFFIX="$(date +%Y%m%d%H%M%S)"
EMAIL="e2e-$SUFFIX@example.com"
TAG="e2e-$SUFFIX"
EVENT_ID="$(cat /proc/sys/kernel/random/uuid 2>/dev/null || uuidgen)"
OCCURRED_AT="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

step "subscribing $EMAIL to $REPO"
SUBSCRIBE_STATUS="$(curl -sS -o /tmp/subber-kafka-e2e-subscribe.out -w "%{http_code}" \
  -X POST "$BASE_URL/api/subscribe" \
  -H "Content-Type: application/json" \
  -H "X-API-Key: $API_KEY" \
  --data "{\"email\":\"$EMAIL\",\"repo\":\"$REPO\"}")"
if [ "$SUBSCRIBE_STATUS" != "200" ]; then
  cat /tmp/subber-kafka-e2e-subscribe.out >&2 || true
  echo "subscribe returned $SUBSCRIBE_STATUS" >&2
  exit 1
fi
echo "OK: subscribe"

step "reading confirmation token from subscription-api database"
TOKEN="$(wait_value "confirmation token" \
  "docker compose -f '$COMPOSE_FILE' exec -T postgres-api psql -U postgres -d subber_api -t -A -c \"SELECT token FROM subscriptions WHERE email='$EMAIL' AND repo='$REPO' LIMIT 1;\"" \
  "[ -n \"\$value\" ]")"

step "confirming subscription"
CONFIRM_STATUS="$(curl -sS -o /tmp/subber-kafka-e2e-confirm.out -w "%{http_code}" "$BASE_URL/api/confirm/$TOKEN")"
if [ "$CONFIRM_STATUS" != "200" ]; then
  cat /tmp/subber-kafka-e2e-confirm.out >&2 || true
  echo "confirm returned $CONFIRM_STATUS" >&2
  exit 1
fi
echo "OK: confirm"

step "waiting for scanner watchlist update through Kafka"
wait_value "scanner watchlist row" \
  "docker compose -f '$COMPOSE_FILE' exec -T postgres-scanner psql -U postgres -d subber_scanner -t -A -c \"SELECT COUNT(*) FROM scanner_watchlist WHERE repo='$REPO';\"" \
  "[ \"\${value:-0}\" -gt 0 ]" >/dev/null

step "publishing ReleaseDetected to Kafka"
EVENT="{\"event_id\":\"$EVENT_ID\",\"event_type\":\"ReleaseDetected\",\"occurred_at\":\"$OCCURRED_AT\",\"source\":\"scripted-kafka-e2e\",\"correlation_id\":\"$EVENT_ID\",\"payload\":{\"repo\":\"$REPO\",\"tag\":\"$TAG\",\"url\":\"https://github.com/$REPO/releases/tag/$TAG\"}}"
printf '%s|%s\n' "$REPO" "$EVENT" |
  docker compose -f "$COMPOSE_FILE" exec -T kafka /opt/kafka/bin/kafka-console-producer.sh \
    --bootstrap-server localhost:9092 \
    --topic subber.release.events \
    --property parse.key=true \
    --property key.separator='|'
echo "OK: release event published"

step "waiting for notification-service delivery"
STATUS="$(wait_value "release notification sent" \
  "docker compose -f '$COMPOSE_FILE' exec -T postgres-notifier psql -U postgres -d subber_notifier -t -A -c \"SELECT status FROM notification_deliveries WHERE recipient_email='$EMAIL' AND repo='$REPO' AND tag='$TAG' ORDER BY updated_at DESC LIMIT 1;\"" \
  "[ \"\$value\" = sent ]")"
if [ "$STATUS" != "sent" ]; then
  echo "notification status is $STATUS" >&2
  exit 1
fi

step "checking Mailpit API"
curl -fsS "http://localhost:8025/api/v1/messages" >/dev/null

echo "Kafka E2E OK: email=$EMAIL repo=$REPO tag=$TAG"
