const crypto = require("node:crypto");
const fs = require("node:fs");
const http = require("node:http");
const https = require("node:https");

const baseURL = process.env.BASE_URL || "http://localhost:8080";
const apiKey = process.env.API_KEY || "";
const requests = Number.parseInt(process.env.REQUESTS || "100", 10);
const concurrency = Number.parseInt(process.env.CONCURRENCY || "10", 10);
const transport = process.env.NOTIFICATION_TRANSPORT || "unknown";
const output = process.env.OUTPUT || "";
const repos = (process.env.REPOS || "torvalds/linux,golang/go,microsoft/vscode,facebook/react,vuejs/vue")
  .split(",")
  .map((repo) => repo.trim())
  .filter(Boolean);

function percentile(values, p) {
  if (values.length === 0) {
    return 0;
  }
  const index = Math.min(values.length - 1, Math.ceil((p / 100) * values.length) - 1);
  return values[index];
}

function varintSize(value) {
  let size = 1;
  while (value >= 128) {
    value = Math.floor(value / 128);
    size += 1;
  }
  return size;
}

function protobufStringFieldSize(fieldNumber, value) {
  if (!value) {
    return 0;
  }
  const length = Buffer.byteLength(value);
  const key = fieldNumber << 3 | 2;
  return varintSize(key) + varintSize(length) + length;
}

function representativePayloadSizes() {
  const payload = {
    notification_id: "11111111-1111-4111-8111-111111111111",
    idempotency_key: "confirmation:owner/repo:emailhash:messagehash",
    recipient_email: "user@example.com",
    email_hash: "emailhash1234",
    repo: "owner/repo",
    tag: "",
    message: "Welcome! Please confirm your subscription to GitHub repository updates by clicking here: http://localhost:8080/api/confirm/token",
    correlation_id: "22222222-2222-4222-8222-222222222222",
  };
  const kafkaEnvelope = {
    event_id: payload.notification_id,
    event_type: "NotificationSendRequested",
    occurred_at: "2026-06-25T00:00:00Z",
    source: "subscription-api",
    correlation_id: payload.correlation_id,
    payload: {
      notification_id: payload.notification_id,
      idempotency_key: payload.idempotency_key,
      recipient_email: payload.recipient_email,
      email_hash: payload.email_hash,
      repo: payload.repo,
      tag: payload.tag,
      message: payload.message,
    },
  };
  const grpcBytes =
    protobufStringFieldSize(1, payload.notification_id) +
    protobufStringFieldSize(2, payload.idempotency_key) +
    protobufStringFieldSize(3, payload.recipient_email) +
    protobufStringFieldSize(4, payload.email_hash) +
    protobufStringFieldSize(5, payload.repo) +
    protobufStringFieldSize(6, payload.tag) +
    protobufStringFieldSize(7, payload.message) +
    protobufStringFieldSize(8, payload.correlation_id);

  return {
    kafkaJsonEnvelopeBytes: Buffer.byteLength(JSON.stringify(kafkaEnvelope)),
    grpcProtobufRequestBytes: grpcBytes,
  };
}

function postSubscribe(index) {
  const repo = repos[index % repos.length];
  const body = JSON.stringify({
    email: `load_${transport}_${Date.now()}_${index}_${crypto.randomUUID().slice(0, 8)}@example.com`,
    repo,
  });
  const url = new URL("/api/subscribe", baseURL);
  const client = url.protocol === "https:" ? https : http;
  const started = process.hrtime.bigint();

  return new Promise((resolve) => {
    const req = client.request(
      url,
      {
        method: "POST",
        headers: {
          "content-type": "application/json",
          "content-length": Buffer.byteLength(body),
          ...(apiKey ? { "x-api-key": apiKey } : {}),
        },
      },
      (res) => {
        let responseBytes = 0;
        res.on("data", (chunk) => {
          responseBytes += chunk.length;
        });
        res.on("end", () => {
          const durationMs = Number(process.hrtime.bigint() - started) / 1e6;
          resolve({
            status: res.statusCode || 0,
            durationMs,
            requestBytes: Buffer.byteLength(body),
            responseBytes,
            ok: res.statusCode >= 200 && res.statusCode < 300,
          });
        });
      },
    );
    req.on("error", () => {
      const durationMs = Number(process.hrtime.bigint() - started) / 1e6;
      resolve({
        status: 0,
        durationMs,
        requestBytes: Buffer.byteLength(body),
        responseBytes: 0,
        ok: false,
      });
    });
    req.write(body);
    req.end();
  });
}

async function run() {
  const results = [];
  let next = 0;
  const started = process.hrtime.bigint();

  async function worker() {
    while (next < requests) {
      const index = next;
      next += 1;
      results.push(await postSubscribe(index));
    }
  }

  await Promise.all(Array.from({ length: Math.min(concurrency, requests) }, () => worker()));

  const totalDurationMs = Number(process.hrtime.bigint() - started) / 1e6;
  const durations = results.map((result) => result.durationMs).sort((a, b) => a - b);
  const failed = results.filter((result) => !result.ok).length;
  const totalRequestBytes = results.reduce((sum, result) => sum + result.requestBytes, 0);
  const totalResponseBytes = results.reduce((sum, result) => sum + result.responseBytes, 0);
  const internalPayload = representativePayloadSizes();
  const summary = {
    transport,
    baseURL,
    requests,
    concurrency,
    totalDurationMs,
    requestsPerSecond: requests / (totalDurationMs / 1000),
    avgLatencyMs: durations.reduce((sum, value) => sum + value, 0) / Math.max(durations.length, 1),
    p95LatencyMs: percentile(durations, 95),
    p99LatencyMs: percentile(durations, 99),
    failedRequests: failed,
    successRequests: requests - failed,
    avgRequestPayloadBytes: totalRequestBytes / Math.max(results.length, 1),
    avgResponsePayloadBytes: totalResponseBytes / Math.max(results.length, 1),
    sampleKafkaJsonEnvelopeBytes: internalPayload.kafkaJsonEnvelopeBytes,
    sampleGrpcProtobufRequestBytes: internalPayload.grpcProtobufRequestBytes,
    statuses: results.reduce((acc, result) => {
      acc[result.status] = (acc[result.status] || 0) + 1;
      return acc;
    }, {}),
  };

  if (output) {
    fs.mkdirSync(require("node:path").dirname(output), { recursive: true });
    fs.writeFileSync(output, `${JSON.stringify(summary, null, 2)}\n`);
  }

  console.log(JSON.stringify(summary, null, 2));
}

run().catch((err) => {
  console.error(err);
  process.exit(1);
});
