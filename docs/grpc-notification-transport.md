# gRPC Notification Transport

This project keeps two notification command transports side by side:

| Mode | Path |
| --- | --- |
| `kafka` | `subscription-api -> outbox -> Kafka -> notification-service` |
| `grpc` | `subscription-api -> gRPC -> notification-service` |

The gRPC path is intentionally direct. It does not write the notification command to the outbox, because that would keep the old asynchronous delivery semantics and hide the difference between the transports.

## Contract Location

The protobuf contract is shared by both services:

```text
api/proto/notification/v1/notification.proto
```

Generated Go code lives in:

```text
pkg/gen/notification/v1
```

`subscription-api` imports the generated client. `notification-service` imports the generated server interface.

## Buf

Buf configuration lives next to the protobuf contracts:

```text
api/proto/buf.yaml
api/proto/buf.gen.yaml
```

Run protobuf checks and generation from `api/proto`:

```sh
cd api/proto
buf lint
buf generate
```

`buf lint` checks the `.proto` contract. `buf generate` refreshes the generated Go files under `pkg/gen`.

## Transport Configuration

Use the same `NOTIFICATION_TRANSPORT` value for the notification scenario on both services:

```text
NOTIFICATION_TRANSPORT=kafka
```

or:

```text
NOTIFICATION_TRANSPORT=grpc
NOTIFICATION_GRPC_ADDR=notification-service:9093
GRPC_PORT=9093
```

Do not compare a mixed setup where `subscription-api` sends through Kafka while `notification-service` is configured for gRPC, or the reverse. That is not a valid transport comparison.

In `kafka` mode, `notification-service` consumes initial notification commands from `subber.notification.commands`. In `grpc` mode, it accepts initial notification commands through `NotificationService/SendNotification`. Retry topics remain enabled in both modes because they are part of the notification-service retry mechanism after a delivery attempt fails.

## Error Handling

The gRPC server validates required request fields and returns `InvalidArgument` for malformed requests. Processing failures are returned as `Internal`. Transport failures from the gRPC client are propagated to the caller so the HTTP subscription request can fail instead of silently losing the notification command.

## Trade-offs

Kafka/outbox keeps a durable notification intent in the same database transaction as the subscription write. That is better when notification-service is temporarily unavailable. The gRPC mode is a direct unary call, so it has less transport overhead and simpler request/response semantics, but it depends on notification-service availability during the subscription request and does not persist a separate outbox command.

## Comparing Transports

Run the A/B comparison script from the repository root:

```powershell
powershell -ExecutionPolicy Bypass -File scripts/compare-notification-transports.ps1 -Build -Requests 200 -Concurrency 20
```

The script runs the compose stack in `kafka` mode, executes the same HTTP subscribe load, then recreates the stack in `grpc` mode and repeats the load. It stores JSON summaries in:

```text
tmp/notification-transport-results/kafka.json
tmp/notification-transport-results/grpc.json
```

The summary includes request count, requests per second, average latency, p95/p99 latency, failed requests, average HTTP request/response payload bytes, and representative internal transport payload sizes:

| Field | Meaning |
| --- | --- |
| `sampleKafkaJsonEnvelopeBytes` | representative Kafka JSON envelope size for a notification command |
| `sampleGrpcProtobufRequestBytes` | representative protobuf request size for the same notification command |

The runs are sequential rather than parallel so both modes do not compete for the same host ports, databases, Kafka broker, and Mailpit instance.
