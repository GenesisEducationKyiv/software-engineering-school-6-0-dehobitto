## ADR-003: Select log transport mechanism between application and Logstash

**Status:** Accepted  
**Date:** 2026-05-30  
**Author:** Oleksandr Makarov

## Context

* The application must ship structured JSON logs to a Logstash instance running in Docker Compose for indexing in Elasticsearch.
* Log delivery reliability is a first-class requirement: logs must not be silently dropped if Logstash is temporarily unavailable (restart, overload, deployment).
* Three transport strategies were evaluated: direct TCP connection, Redis as an intermediary queue, and RabbitMQ as a dedicated message broker.

## Variants considered

**1. Direct TCP to Logstash (no broker)**

* **Positives:** zero additional infrastructure; simplest configuration - logrus sends JSON over a TCP socket directly to Logstash's `tcp` input; easy to reason about.
* **Negatives:** no delivery guarantee - if Logstash is down at the moment of the log call, the message is lost; the application is tightly coupled to Logstash availability; no buffering between producer and consumer.

**2. Redis as message queue**

* **Positives:** Redis is already present in the infrastructure (used as GitHub API cache), so no new service is required; Logstash has a native `redis` input plugin that consumes from a Redis list via `BRPOP`; logs are buffered in the list if Logstash is temporarily unavailable.
* **Negatives:** Redis currently serves as the GitHub API response cache - adding log queuing introduces a second responsibility on the same instance; a single Redis failure simultaneously disables caching and log buffering, creating a compound failure mode; unbounded log accumulation can exhaust Redis memory and degrade or crash the cache layer; Redis lists provide no per-message acknowledgement - if Logstash crashes mid-batch, messages already popped from the list are lost.

**3. RabbitMQ**

* **Positives:** purpose-built for reliable message delivery; supports consumer acknowledgements (Logstash confirms receipt before the message is removed from the queue), durable queues (queue and messages survive broker restart), and dead letter exchanges (unprocessable messages are routed to a separate queue rather than dropped); fully decouples log producers from Logstash availability; memory pressure on the broker does not affect application caching.
* **Negatives:** introduces a new service dependency in Docker Compose; operationally heavier than a Redis list for the same buffering function.

## Final choice

**RabbitMQ selected.**

The decisive factors are delivery reliability and separation of concerns. Direct TCP provides no buffering and is unsuitable when log loss is unacceptable. Redis was ruled out because it already carries the GitHub API cache responsibility: a single Redis failure would simultaneously degrade caching and lose the log buffer, and unbounded queue growth could corrupt the cache memory budget. RabbitMQ is the correct tool for this role - its acknowledgement model ensures a log entry is retained in the queue until Logstash successfully processes it, and durable queues survive broker restarts without message loss. The additional Docker Compose service is an acceptable cost for the isolation and reliability guarantees it provides.

## Implementation Details

* **Broker:** RabbitMQ 3.x with management plugin (`rabbitmq:3-management`) added to `docker-compose.yml`.
* **Queue:** `logs`, declared durable with persistent message delivery mode so messages survive broker restart.
* **Producer:** logrus AMQP hook serializes each log entry as JSON and enqueues it into a bounded in-process buffer. A dedicated background publisher sends buffered entries to RabbitMQ via AMQP.
* **Consumer:** Logstash `rabbitmq` input plugin subscribes to the `logs` queue with `ack => true` so messages are acknowledged only after successful processing.
* **Dead letter exchange:** `logs.dlx` exchange bound to `logs.dead` queue; messages that Logstash cannot process are routed there rather than dropped.
* **Logstash pipeline:** `rabbitmq input → (no filter needed, messages are already valid JSON) → elasticsearch output`.
* **Backpressure policy:** logging must not block application request handling indefinitely. If the in-process buffer is full, the hook drops the log entry and writes a short warning to stderr. On application shutdown the hook drains buffered messages with a bounded timeout before closing the AMQP connection.
* **Metrics:** the hook records Prometheus counters for enqueued, dropped, published, and failed publish attempts.
* **Retention:** Elasticsearch setup applies a `subber-logs-7d` ILM policy and index template to keep Subber log indices for 7 days.

## Consequences

### Positives

* Logs are guaranteed to reach Elasticsearch even if Logstash restarts or lags behind.
* Redis remains dedicated to GitHub API caching with a predictable memory budget.
* The application has no runtime dependency on Logstash - it only requires RabbitMQ to be reachable.
* Dead letter queue provides an audit trail of any messages that failed processing.
* RabbitMQ publishing latency is kept off the request path in the steady state.
* Log delivery degradation is visible in Prometheus and Grafana through explicit logging-pipeline counters.

### Negatives

* One additional service in Docker Compose (RabbitMQ).
* The bounded in-process buffer can drop logs during sustained broker outages or log spikes. Dropping logs is preferred over unbounded memory growth or request latency amplification.
* RabbitMQ management and health monitoring adds operational overhead compared to a simple TCP socket.
* A 7-day retention window is a local/default operational choice; production deployments may need a longer retention policy or rollover based on compliance, cost, and traffic volume.
