# Architecture Layers

Subber keeps service ownership explicit and keeps cross-service communication behind contracts and messaging. The layer rules below are intentionally simple enough to be enforced by tests.

## Layer Distribution

| Layer | Paths | Responsibility | May depend on |
| --- | --- | --- | --- |
| Entry points | `services/*/main.go`, `services/*/cmd/*` | Runtime composition, process configuration, worker startup | Same service internals, `pkg` |
| Inbound adapters | `services/*/internal/httpapi`, `services/*/internal/grpcapi` | HTTP/gRPC transport, request/response mapping, auth and metrics endpoints | Same service application packages, `pkg` |
| Application and domain | `services/subscription-api/internal/subscription`, `services/subscription-api/internal/watchsaga`, `services/scanner-service/internal/scanner`, `services/notification-service/internal/delivery` | Use cases, domain state transitions, ports/interfaces, event handling | Standard library, third-party libraries, `pkg`, same service utilities |
| Outbound adapters | `services/*/internal/email`, `services/*/internal/github`, `services/*/internal/cache`, repository implementations co-located with service packages | External systems, persistence, cache, SMTP, GitHub API | Same service application contracts, `pkg` |
| Service infrastructure | `services/*/internal/config`, `services/*/internal/dbmigrations` | Service-local configuration and database schema | `pkg`, embedded SQL |
| Shared infrastructure and contracts | `pkg` | Kafka, outbox, logging, migrations, env/config helpers, generated/shared contracts | Standard library and external libraries only |
| Cross-cutting tests | `tests/integration`, `tests/e2e`, `tests/architecture` | Integration, E2E, and architectural guardrails | Public/shared packages and test targets |

## Dependency Rules

1. A service may import its own `internal` packages and shared packages from `pkg`.
2. A service must not import another service module directly.
3. Shared packages under `pkg` must not import service-owned code.
4. Application/domain packages must not import inbound adapters, process entry points, service config, or database migration packages.
5. Business communication between services goes through `pkg/contracts`, Kafka/outbox, or explicit external transports documented in ADRs.

These rules are checked by `tests/architecture`. When a new service or layer is added, update the layer table and the test rules in the same change.
