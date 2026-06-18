param(
    [string]$AppUrl = "http://localhost:8080",
    [string]$PrometheusUrl = "http://localhost:9090",
    [string]$ElasticsearchUrl = "http://localhost:9200",
    [string]$GrafanaUrl = "http://localhost:3000",
    [string]$KibanaUrl = "http://localhost:5601",
    [int]$TimeoutSeconds = 90
)

$ErrorActionPreference = "Stop"

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
            Start-Sleep -Seconds 3
        }
    } while ($true)
}

function Invoke-Json {
    param(
        [string]$Uri,
        [string]$Method = "GET",
        [object]$Body = $null
    )

    $params = @{
        Uri = $Uri
        Method = $Method
        Headers = @{ "Content-Type" = "application/json" }
    }
    if ($null -ne $Body) {
        $params.Body = ($Body | ConvertTo-Json -Depth 10)
    }
    Invoke-RestMethod @params
}

Wait-Until "application metrics endpoint" {
    $metrics = Invoke-WebRequest -Uri "$AppUrl/metrics" -UseBasicParsing
    if ($metrics.StatusCode -ne 200 -or -not $metrics.Content.Contains("http_requests_total")) {
        throw "metrics endpoint not ready"
    }
} $TimeoutSeconds

Wait-Until "Prometheus target" {
    $response = Invoke-Json "$PrometheusUrl/api/v1/query?query=up%7Bjob%3D%22subber%22%7D"
    if ($response.status -ne "success" -or $response.data.result.Count -eq 0 -or $response.data.result[0].value[1] -ne "1") {
        throw "subber target is not UP"
    }
} $TimeoutSeconds

Wait-Until "RabbitMQ Prometheus target" {
    $response = Invoke-Json "$PrometheusUrl/api/v1/query?query=up%7Bjob%3D%22rabbitmq%22%7D"
    if ($response.status -ne "success" -or $response.data.result.Count -eq 0 -or $response.data.result[0].value[1] -ne "1") {
        throw "rabbitmq target is not UP"
    }
} $TimeoutSeconds

Wait-Until "Prometheus alert rules" {
    $response = Invoke-Json "$PrometheusUrl/api/v1/rules"
    $rulesJson = $response | ConvertTo-Json -Depth 20
    if (-not $rulesJson.Contains("SubberTargetDown") -or -not $rulesJson.Contains("SubberLogEntriesDropped") -or -not $rulesJson.Contains("SubberLogDeadLetterQueueNotEmpty")) {
        throw "expected alert rules not loaded"
    }
} $TimeoutSeconds

Wait-Until "Grafana health" {
    $response = Invoke-Json "$GrafanaUrl/api/health"
    if ($response.database -ne "ok") {
        throw "grafana database is not ok"
    }
} $TimeoutSeconds

Wait-Until "Kibana status" {
    $response = Invoke-Json "$KibanaUrl/api/status"
    $statusJson = $response | ConvertTo-Json -Depth 20
    if (-not $statusJson.Contains("available")) {
        throw "kibana is not available"
    }
} $TimeoutSeconds

Wait-Until "Elasticsearch ILM policy" {
    $policy = Invoke-Json "$ElasticsearchUrl/_ilm/policy/subber-logs-7d"
    if ($null -eq $policy."subber-logs-7d") {
        throw "subber-logs-7d policy missing"
    }
} $TimeoutSeconds

$requestId = "smoke-$([DateTimeOffset]::UtcNow.ToUnixTimeSeconds())"
$token = "00000000-0000-0000-0000-000000000000"
$request = $null
try {
    $request = Invoke-WebRequest -Uri "$AppUrl/api/confirm/$token" -Headers @{ "X-Request-ID" = $requestId } -UseBasicParsing
} catch {
    $request = $_.Exception.Response
}
if ($request.StatusCode -ne 404) {
    throw "expected app request to return 404, got $($request.StatusCode)"
}

Wait-Until "request metric in Prometheus" {
    $query = "http_requests_total{route=`"/api/confirm/:token`",status_code=`"404`"}"
    $encoded = [uri]::EscapeDataString($query)
    $response = Invoke-Json "$PrometheusUrl/api/v1/query?query=$encoded"
    if ($response.status -ne "success" -or $response.data.result.Count -eq 0) {
        throw "expected request metric not found"
    }
} $TimeoutSeconds

Wait-Until "request log in Elasticsearch" {
    $body = @{
        size = 1
        query = @{
            bool = @{
                must = @(
                    @{ match_phrase = @{ request_id = $requestId } },
                    @{ match_phrase = @{ component = "http" } }
                )
            }
        }
    }
    $response = Invoke-Json "$ElasticsearchUrl/subber-logs-*/_search" "POST" $body
    if ($response.hits.total.value -lt 1) {
        throw "expected request log not found"
    }
} $TimeoutSeconds

Write-Host "Observability smoke test passed for request_id=$requestId"
