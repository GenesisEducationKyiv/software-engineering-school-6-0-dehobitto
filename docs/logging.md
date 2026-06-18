# Logging

Subber uses structured JSON logs and ships them through a brokered pipeline to Elasticsearch for search and Kibana dashboards.

## Stack

```text
Subber JSON logs
  -> async Logrus RabbitMQ hook
  -> RabbitMQ queue logs
  -> Logstash RabbitMQ input
  -> Elasticsearch index subber-logs-YYYY.MM.dd
  -> Kibana
```

RabbitMQ provides buffering between the application and Logstash. If Logstash is down or slow, log entries can accumulate in RabbitMQ instead of coupling application request handling to Logstash availability.

## Format

Every application log entry is a single JSON object.

Common fields:

| Field | Description |
| --- | --- |
| `time` | event timestamp |
| `level` | log level, for example `info`, `warning`, `error` |
| `msg` | event message |
| `component` | application component, for example `http`, `handler`, `service`, `github`, `scanner`, `notifier` |
| `request_id` | correlation id for HTTP request workflows |
| `scan_cycle_id` | correlation id for background scanner workflows |
| `repo` | GitHub repository name |
| `error` | error message when an operation fails |

HTTP request log fields:

| Field | Description |
| --- | --- |
| `method` | HTTP method |
| `route` | Gin route template, for example `/api/confirm/:token` |
| `status_code` | HTTP response status |
| `duration_ms` | request duration in milliseconds |
| `has_query` | whether a query string was present |
| `user_agent` | request user agent |
| `client_ip_hash` | hashed client IP |

PII-safe fields:

| Field | Description |
| --- | --- |
| `email_hash` | stable hash of email for correlation without raw email |
| `client_ip_hash` | stable hash of IP for correlation without raw IP |

## Sensitive Data Rules

Never log:

- credentials;
- API keys;
- GitHub tokens;
- SMTP passwords;
- confirmation tokens;
- unsubscribe tokens;
- raw request bodies;
- raw query strings;
- raw email addresses;
- raw client IP addresses.

Tokens can appear in user-facing URLs, but logs must use route templates like `/api/confirm/:token`, not raw paths.

## Correlation

HTTP requests use `X-Request-ID`.

Flow:

1. If the request has `X-Request-ID`, Subber validates and reuses it.
2. If it is missing or unsafe, Subber generates a new UUID.
3. The id is written to the response header.
4. The id is stored in `context.Context`.
5. Downstream handler/service/GitHub logs use the same `request_id`.
6. Confirmation notification jobs created from HTTP requests carry the originating `request_id`.

Background scanner work is not attached to an HTTP request. It uses `scan_cycle_id`, generated once per scanner cycle and propagated to scanner/notifier logs.

## RabbitMQ Topology

The application declares:

- queue `logs`;
- direct dead-letter exchange `logs.dlx`;
- dead-letter queue `logs.dead`.

Messages are persistent. The main queue is durable. Logstash consumes `logs` with acknowledgements enabled.

If the async in-process logging buffer is full, Subber drops the log entry and increments `log_entries_dropped_total`. Dropping logs is preferred over unbounded memory growth or blocking application requests indefinitely.

## Monitoring

Application logging counters:

- `log_entries_enqueued_total`;
- `log_entries_dropped_total`;
- `log_entries_published_total`;
- `log_publish_errors_total`.

RabbitMQ queue alerts:

- `SubberLogDeadLetterQueueNotEmpty` fires when `logs.dead` has ready messages;
- `SubberLogQueueBacklog` fires when `logs` has more than 1000 ready messages for 5 minutes.

## Retention

Elasticsearch setup applies:

- ILM policy: `subber-logs-7d`;
- index template: `subber-logs-template`;
- index pattern: `subber-logs-*`.

Subber log indices are retained for 7 days in the local stack.
