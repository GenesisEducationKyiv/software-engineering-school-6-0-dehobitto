param(
    [string]$ComposeFile = "compose.microservices.yml",
    [switch]$StartStack,
    [switch]$Build,
    [int]$TimeoutSeconds = 90
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
        [int]$TimeoutSeconds = 90
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
    return $response
}

Write-Step "validating compose config"
docker compose -f $ComposeFile config --quiet

if ($StartStack) {
    Write-Step "starting compose stack"
    if ($Build) {
        docker compose -f $ComposeFile up --build -d
    } else {
        docker compose -f $ComposeFile up -d
    }
}

Write-Step "checking service status"
docker compose -f $ComposeFile ps

Wait-Until "subscription-api root" {
    Invoke-HttpOk "http://localhost:8080/" | Out-Null
} $TimeoutSeconds

Wait-Until "subscription-api metrics" {
    Invoke-HttpOk "http://localhost:8080/metrics" | Out-Null
} $TimeoutSeconds

Wait-Until "scanner-service metrics" {
    Invoke-HttpOk "http://localhost:8081/metrics" | Out-Null
} $TimeoutSeconds

Wait-Until "notification-service metrics" {
    Invoke-HttpOk "http://localhost:8082/metrics" | Out-Null
} $TimeoutSeconds

Wait-Until "mailpit api" {
    Invoke-HttpOk "http://localhost:8025/api/v1/messages" | Out-Null
} $TimeoutSeconds

Wait-Until "elasticsearch health" {
    $health = Invoke-RestMethod -Uri "http://localhost:9200/_cluster/health" -TimeoutSec 10
    if ($health.status -notin @("green", "yellow")) {
        throw "unexpected elasticsearch status $($health.status)"
    }
} $TimeoutSeconds

Wait-Until "kibana ready" {
    $response = Invoke-RestMethod -Uri "http://localhost:5601/api/status" -TimeoutSec 10
    if ($response.status.level -notin @("available", "degraded")) {
        throw "unexpected kibana status $($response.status.level)"
    }
} $TimeoutSeconds

Wait-Until "prometheus ready" {
    Invoke-HttpOk "http://localhost:9090/-/ready" | Out-Null
} $TimeoutSeconds

Wait-Until "prometheus service targets" {
    $response = Invoke-RestMethod -Uri "http://localhost:9090/api/v1/targets?state=active" -TimeoutSec 10
    if ($response.status -ne "success") {
        throw "prometheus targets query failed"
    }
    $targets = @{}
    foreach ($target in $response.data.activeTargets) {
        $targets[$target.labels.job] = $target.health
    }
    foreach ($job in @("subscription-api", "scanner-service", "notification-service")) {
        if ($targets[$job] -ne "up") {
            throw "$job target is not up"
        }
    }
} $TimeoutSeconds

Wait-Until "grafana ready" {
    $response = Invoke-RestMethod -Uri "http://localhost:3000/api/health" -TimeoutSec 10
    if ($response.database -ne "ok") {
        throw "grafana database is not ok"
    }
} $TimeoutSeconds

Wait-Until "grafana prometheus datasource" {
    $response = Invoke-RestMethod -Uri "http://localhost:3000/api/datasources/name/Prometheus" -TimeoutSec 10
    if ($response.type -ne "prometheus" -or $response.url -ne "http://prometheus:9090") {
        throw "prometheus datasource not provisioned"
    }
} $TimeoutSeconds

Wait-Until "grafana subber dashboard" {
    $response = Invoke-RestMethod -Uri "http://localhost:3000/api/dashboards/uid/subber-overview" -TimeoutSec 10
    if ($response.dashboard.title -ne "Subber Overview") {
        throw "subber dashboard not provisioned"
    }
} $TimeoutSeconds

Write-Step "checking kafka topics"
$expectedTopics = @(
    "subber.watchlist.events",
    "subber.release.events",
    "subber.notification.commands",
    "subber.notification.retry.1m",
    "subber.notification.retry.10m",
    "subber.notification.dlq"
)
$topics = docker compose -f $ComposeFile exec -T kafka /opt/kafka/bin/kafka-topics.sh --bootstrap-server localhost:9092 --list
foreach ($topic in $expectedTopics) {
    if ($topics -notcontains $topic) {
        throw "missing kafka topic $topic"
    }
}
Write-Host "OK: kafka topics"

Write-Step "checking vector to elasticsearch log path"
$smokeID = "runtime-smoke-$([DateTimeOffset]::UtcNow.ToUnixTimeSeconds())"
$log = @{
    time = (Get-Date).ToUniversalTime().ToString("o")
    level = "info"
    msg = "runtime smoke log"
    service = "runtime-smoke"
    component = "script"
    smoke_id = $smokeID
} | ConvertTo-Json -Compress
Invoke-RestMethod -Method Post -Uri "http://localhost:8686" -ContentType "application/json" -Body $log | Out-Null

Wait-Until "vector log indexed in elasticsearch" {
    $body = @{
        size = 1
        query = @{ match_phrase = @{ smoke_id = $smokeID } }
    } | ConvertTo-Json -Depth 10
    $response = Invoke-RestMethod -Method Post -Uri "http://localhost:9200/subber-logs-*/_search" -ContentType "application/json" -Body $body -TimeoutSec 10
    if ($response.hits.total.value -lt 1) {
        throw "smoke log not indexed yet"
    }
} $TimeoutSeconds

Write-Host "Runtime smoke OK"
