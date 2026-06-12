## ADR-002: Select structured logging library

**Status:** Accepted; transport updated by ADR-007
**Date:** 2026-05-30  
**Author:** Oleksandr Makarov

## Context

* The application currently uses the standard `log` package which produces unstructured plain-text output (e.g. `2025/05/30 12:00:00 Email sent to user@gmail.com`).
* Introducing an ELK pipeline (Logstash - Elasticsearch - Kibana) requires logs to be machine-parseable - each field must be individually queryable and filterable.
* A structured logging library must output JSON and support attaching arbitrary key-value fields to a logger instance so that context can be propagated through the call chain without repeating it at every call site.
* The library will be used across handlers, workers, services, and infrastructure layers - so the API ergonomics matter for consistency.

## Variants considered

**1. `slog` (standard library, Go 1.21+)**

* **Positives:** zero additional dependencies; built into Go 1.21+ (project uses go 1.26); JSON handler available out of the box.
* **Negatives:** API uses variadic key-value pairs (`slog.Info("msg", "key", val)`) which are weakly typed and easy to misuse; no built-in `WithFields` concept - context propagation requires wrapping in `slog.Logger` with `With()`, which is less discoverable.

**2. `logrus`**

* **Positives:** already present in `go.mod` as an indirect dependency (via testcontainers) - promoting to direct adds no new module; `WithFields(logrus.Fields{...})` API makes structured context explicit and readable; JSON formatter is a single-line opt-in; widely documented and understood.
* **Negatives:** the project is in maintenance mode (no new features); slightly slower than `zap` due to reflection-based field handling, though irrelevant at this service's throughput.

**3. `zap` (Uber)**

* **Positives:** highest throughput of the three; strongly typed field constructors (`zap.String(...)`, `zap.Int(...)`) prevent accidental untyped values.
* **Negatives:** verbose API increases boilerplate at every call site; requires adding a new module dependency; typed fields add friction when the team is not yet familiar with the library.

## Final choice

**logrus selected.**

The decisive factor is that `logrus` is already an indirect dependency in the module graph, making the promotion to direct dependency a zero-cost addition. Its `WithFields` API produces readable, self-documenting log calls and naturally supports the context-propagation pattern needed in this codebase - a logger enriched with `repo` or `email` can be passed through the scanner or notifier worker without re-specifying fields at every call site. The maintenance-mode concern is not material here: the API is stable and the project does not require new logrus features.

## Implementation Details

* **Formatter:** `logrus.JSONFormatter` - outputs one JSON object per line for stdout, optional file duplication, and Vector ingestion.
* **Output:** `os.Stdout` by default; optional `LOG_FILE` appends to a local file opened with `0600` permissions.
* **Transport:** services push logs to Vector by default through the shared logger setup described in ADR-007. RabbitMQ and Logstash are not used in the target architecture.
* **Logger initialisation:** the global logrus instance is configured once in `main`; a small project-local `logger.Logger` interface is then injected into handlers, services, workers, middleware, and infrastructure clients.
* **Context propagation:** components receive a logger pre-enriched with their `component` field at construction time. Request-scoped code adds `request_id` from `context.Context`; notification jobs carry the originating `request_id` when they are created from an HTTP request. Background scanner work uses `scan_cycle_id` for correlation.
* **PII handling:** raw email addresses and client IPs are not written to logs. Email and IP values are converted to stable SHA-256 based short hashes for correlation. Request tokens and raw query strings are not logged.
* **Log levels:** `Info` for normal operation events, `Warn` for recoverable anomalies (channel full, cache miss), `Error` for failures that degrade functionality, `Fatal` only in `main` on startup failure.

## Consequences

### Positives

* Every log line is a valid JSON object, so Vector can forward it to Elasticsearch without custom parsing.
* Fields like `level`, `component`, `request_id`, `email_hash`, `repo`, `route`, `status_code`, and `error` become first-class queryable attributes in Kibana.
* Contextual loggers eliminate repeated field specification across a worker's lifetime and make unit tests independent from global logger state.
* No new module dependency introduced.

### Negatives

* `logrus` is in maintenance mode; if the team later adopts `slog`-based tooling, migration will be needed.
* Slightly more verbose than plain `log.Printf` at call sites - a worthwhile trade for structured output.
* Explicit logger injection adds constructor parameters, but the dependency graph is clearer and easier to test.
