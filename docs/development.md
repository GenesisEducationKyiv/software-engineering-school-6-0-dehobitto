# Development

## Local Stack

Start the microservice stack:

```sh
docker compose -f compose.microservices.yml up --build -d
```

Stop it:

```sh
docker compose -f compose.microservices.yml down
```

Validate compose:

```sh
docker compose -f compose.microservices.yml config --quiet
```

## Configuration

Runtime configuration is env-based. The Compose stack loads common values first and service-specific values second.

Common env:

* [deployments/docker/env/common.env.example](../deployments/docker/env/common.env.example)

Service env:

* [services/subscription-api/.env.example](../services/subscription-api/.env.example)
* [services/scanner-service/.env.example](../services/scanner-service/.env.example)
* [services/notification-service/.env.example](../services/notification-service/.env.example)

Code does not carry local runtime defaults. Local values live in the env example files and Compose env configuration.

## Migrations

Each service has a migration command:

```sh
go run ./services/subscription-api/cmd/migrate
go run ./services/scanner-service/cmd/migrate
go run ./services/notification-service/cmd/migrate
```

Docker Compose runs the migration containers before starting app and outbox relay containers.

## API

Protected endpoints require `X-API-Key`. If `API_KEY` is empty, auth is skipped for local development.

| Method | Path | Auth | Description |
| --- | --- | --- | --- |
| `POST` | `/api/subscribe` | yes | Subscribe email to a repo |
| `GET` | `/api/subscriptions/` | yes | List confirmed subscriptions for an email |
| `GET` | `/api/confirm/:token` | no | Confirm a subscription |
| `GET` | `/api/unsubscribe/:token` | no | Unsubscribe |
| `GET` | `/metrics` | no | Prometheus metrics |

## Flow

```text
POST /api/subscribe
-> subscription-api saves unconfirmed subscription
-> subscription-api writes NotificationSendRequested to outbox
-> outbox relay publishes to Kafka
-> notification-service sends confirmation email
-> user confirms subscription
-> subscription-api writes RepoWatchStartRequested
-> scanner-service stores repo in watchlist
-> scanner-service detects ReleaseDetected
-> subscription-api expands release to NotificationSendRequested per subscriber
-> notification-service sends release emails
```
