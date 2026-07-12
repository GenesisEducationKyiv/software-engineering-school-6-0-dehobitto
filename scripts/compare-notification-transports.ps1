param(
    [string]$ComposeFile = "compose.microservices.yml",
    [int]$Requests = 100,
    [int]$Concurrency = 10,
    [int]$TimeoutSeconds = 120,
    [string]$BaseUrl = "http://localhost:8080",
    [string]$OutputDir = "tmp/notification-transport-results",
    [switch]$Build,
    [switch]$KeepStack
)

$ErrorActionPreference = "Stop"

function Write-Step {
    param([string]$Message)
    Write-Host "==> $Message"
}

function Wait-Until {
    param(
        [string]$Name,
        [scriptblock]$Check,
        [int]$TimeoutSeconds = 120
    )

    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    do {
        try {
            & $Check
            Write-Host "OK: $Name"
            return
        } catch {
            if ((Get-Date) -ge $deadline) {
                throw "Timed out waiting for $Name. Last error: $($_.Exception.Message)"
            }
            Start-Sleep -Seconds 2
        }
    } while ($true)
}

function Invoke-HttpOk {
    param([string]$Uri)
    $response = Invoke-WebRequest -Uri $Uri -UseBasicParsing -TimeoutSec 10
    if ($response.StatusCode -lt 200 -or $response.StatusCode -ge 300) {
        throw "$Uri returned $($response.StatusCode)"
    }
}

function Start-StackForTransport {
    param([string]$Transport)

    Write-Step "starting stack in $Transport mode"
    $env:NOTIFICATION_TRANSPORT = $Transport
    $env:NOTIFICATION_GRPC_ADDR = "notification-service:9093"
    $env:NOTIFICATION_GRPC_PORT = "9093"

    docker compose -f $ComposeFile down -v --remove-orphans
    if ($Build) {
        docker compose -f $ComposeFile up --build -d
    } else {
        docker compose -f $ComposeFile up -d
    }

    Wait-Until "subscription-api root" {
        Invoke-HttpOk "$BaseUrl/"
    } $TimeoutSeconds
    Wait-Until "subscription-api metrics" {
        Invoke-HttpOk "$BaseUrl/metrics"
    } $TimeoutSeconds
    Wait-Until "notification-service metrics" {
        Invoke-HttpOk "http://localhost:8082/metrics"
    } $TimeoutSeconds
    Wait-Until "mailpit api" {
        Invoke-HttpOk "http://localhost:8025/api/v1/messages"
    } $TimeoutSeconds
}

function Invoke-TransportLoad {
    param([string]$Transport)

    $resultPath = Join-Path $OutputDir "$Transport.json"
    $env:BASE_URL = $BaseUrl
    $env:REQUESTS = [string]$Requests
    $env:CONCURRENCY = [string]$Concurrency
    $env:OUTPUT = $resultPath
    $env:NOTIFICATION_TRANSPORT = $Transport

    Write-Step "running load test for $Transport mode"
    node scripts/notification-transport-load.js
    return Get-Content $resultPath -Raw | ConvertFrom-Json
}

if (-not (Get-Command node -ErrorAction SilentlyContinue)) {
    throw "Node.js is required for scripts/notification-transport-load.js"
}

New-Item -ItemType Directory -Force -Path $OutputDir | Out-Null

$originalTransport = $env:NOTIFICATION_TRANSPORT
$originalGrpcAddr = $env:NOTIFICATION_GRPC_ADDR
$originalGrpcPort = $env:NOTIFICATION_GRPC_PORT
$originalBaseUrl = $env:BASE_URL
$originalRequests = $env:REQUESTS
$originalConcurrency = $env:CONCURRENCY
$originalOutput = $env:OUTPUT

try {
    $results = @()
    foreach ($transport in @("kafka", "grpc")) {
        Start-StackForTransport $transport
        $results += Invoke-TransportLoad $transport
    }

    Write-Step "comparison summary"
    $results |
        Select-Object transport, requests, concurrency,
            @{Name = "rps"; Expression = { [math]::Round($_.requestsPerSecond, 2) } },
            @{Name = "avg_ms"; Expression = { [math]::Round($_.avgLatencyMs, 2) } },
            @{Name = "p95_ms"; Expression = { [math]::Round($_.p95LatencyMs, 2) } },
            @{Name = "p99_ms"; Expression = { [math]::Round($_.p99LatencyMs, 2) } },
            failedRequests,
            @{Name = "avg_req_bytes"; Expression = { [math]::Round($_.avgRequestPayloadBytes, 2) } },
            @{Name = "avg_resp_bytes"; Expression = { [math]::Round($_.avgResponsePayloadBytes, 2) } },
            sampleKafkaJsonEnvelopeBytes,
            sampleGrpcProtobufRequestBytes |
        Format-Table -AutoSize

    $kafka = $results | Where-Object { $_.transport -eq "kafka" } | Select-Object -First 1
    $grpc = $results | Where-Object { $_.transport -eq "grpc" } | Select-Object -First 1
    if ($kafka -and $grpc) {
        $rpsDelta = $grpc.requestsPerSecond - $kafka.requestsPerSecond
        $p95Delta = $grpc.p95LatencyMs - $kafka.p95LatencyMs
        Write-Host ("gRPC - Kafka RPS delta: {0}" -f [math]::Round($rpsDelta, 2))
        Write-Host ("gRPC - Kafka p95 latency delta, ms: {0}" -f [math]::Round($p95Delta, 2))
    }
} finally {
    if (-not $KeepStack) {
        Write-Step "stopping stack"
        docker compose -f $ComposeFile down -v --remove-orphans
    }

    $env:NOTIFICATION_TRANSPORT = $originalTransport
    $env:NOTIFICATION_GRPC_ADDR = $originalGrpcAddr
    $env:NOTIFICATION_GRPC_PORT = $originalGrpcPort
    $env:BASE_URL = $originalBaseUrl
    $env:REQUESTS = $originalRequests
    $env:CONCURRENCY = $originalConcurrency
    $env:OUTPUT = $originalOutput
}
