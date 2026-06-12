# Logging

Subber services write structured JSON logs to stdout. Log delivery is best-effort: business data and domain events must not depend on the log pipeline.

## Target Pipeline

```text
service stdout
  -> container logs
  -> async Vector sidecar push
  -> Vector HTTP input
  -> Vector Elasticsearch batch buffer
  -> Elasticsearch
```

Kafka, RabbitMQ, and Logstash are not used for log transport in the target microservice architecture.

## Service Behavior

Each service uses the shared `pkg/logger` package.

Default behavior:

* `LOG_LEVEL=info`
* logs are written to stdout as JSON;
* `LOG_SIDECAR_ENABLED=true`;
* `LOG_SIDECAR_URL=http://vector:8686`;
* `LOG_FILE=` and file duplication is disabled.

Vector push can be disabled explicitly:

* set `LOG_SIDECAR_ENABLED=false`;
* the service still writes to stdout.

Optional file duplication:

* set `LOG_FILE=/path/to/service.log`;
* the file is opened with `0600` permissions;
* this is not enabled by default.

If Vector is unavailable, the application should keep serving business traffic. Losing logs is acceptable; losing subscription, scan, or notification data is not.

## Format

Every log entry is a JSON object.

Common fields:

| Field | Description |
| --- | --- |
| `time` | Event timestamp |
| `level` | Log level |
| `msg` | Event message |
| `service` | Service name, for example `subscription-api`, `scanner-service`, `notification-service` |
| `component` | Component name, for example `handler`, `service`, `github`, `delivery` |
| `request_id` | HTTP request correlation id |
| `correlation_id` | Event workflow correlation id |
| `repo` | GitHub repository name |
| `error` | Error message |

PII-safe fields:

| Field | Description |
| --- | --- |
| `email_hash` | Stable hash of email for correlation without raw email |
| `client_ip_hash` | Stable hash of IP for correlation without raw IP |

## Sensitive Data Rules

Never log:

* credentials;
* API keys;
* GitHub tokens;
* SMTP passwords;
* confirmation tokens;
* unsubscribe tokens;
* raw request bodies;
* raw query strings;
* raw email addresses;
* raw client IP addresses.

Tokens can appear in user-facing URLs, but logs should use route templates like `/api/confirm/:token`, not raw paths.

## Correlation

HTTP requests use `X-Request-ID`.

Event workflows use `correlation_id` from the Kafka envelope. `subscription-api` preserves the correlation id when expanding `ReleaseDetected` into `NotificationSendRequested` commands.

## Vector

Vector is configured in [deployments/docker/vector/vector.yaml](../deployments/docker/vector/vector.yaml).

The local compose stack runs Vector as a separate container. Services push logs to it by default, and the push can be disabled with `LOG_SIDECAR_ENABLED=false`.

Vector sends logs to Elasticsearch in batches. The local configuration flushes when either:

* the batch reaches `524288` bytes, approximately `0.5MB`;
* or `5s` passes since the previous flush.

## Legacy Path

The old RabbitMQ/Logstash log transport is historical. It is superseded by ADR-007 and is not part of the target microservice deployment.
