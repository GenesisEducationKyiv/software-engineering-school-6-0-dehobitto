# PR Observability Notes

For this repo, keep user traffic and infrastructure traffic separate.

- `/api/**`: log and metric normally.
- `/metrics`: keep separate from RED and request logs.
- `/static`: do not log as regular request traffic; it is just the hosted frontend page/assets.

Reason: Prometheus scrapes `/metrics` on a fixed interval, so including it in the shared HTTP request path adds constant background noise to request-rate panels and log volume without user value. Static asset requests add similar noise and are usually not useful in the main API observability path.

If a future PR changes the routing or middleware setup, check this file first and keep the same split unless there is a specific reason to measure static/metrics traffic separately.

If a future PR uses an async log hook or buffered publisher, treat `Fatal` paths specially. `logrus.Fatal` calls `os.Exit(1)` immediately after logging, so a queued log entry may never be drained to RabbitMQ. The most valuable crash logs are often the ones at risk of being lost, so fatal logging should either flush synchronously or perform an explicit drain before exit.

If the async publisher does its RabbitMQ write during shutdown, prefer `PublishWithContext` over an unbounded publish call. The point is not deprecation; it is to keep the final drain bounded so a hung broker or network path cannot stall graceful shutdown past the allotted timeout.
