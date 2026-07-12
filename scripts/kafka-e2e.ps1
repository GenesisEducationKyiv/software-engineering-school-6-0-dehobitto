param(
    [string]$ComposeFile = "compose.microservices.yml",
    [string]$BaseUrl = "http://localhost:8080",
    [string]$Repo = "cli/cli",
    [string]$ApiKey = "dev-api-key",
    [int]$TimeoutSeconds = 120
)

$ErrorActionPreference = "Stop"

function Write-Step {
    param([string]$Message)
    Write-Host "==> $Message"
}

function First-NonEmptyLine {
    param([string[]]$Lines)
    $line = $Lines | Where-Object { $_ -and $_.Trim() -ne "" } | Select-Object -First 1
    if ($line) {
        return $line.Trim()
    }
    return ""
}

function Wait-UntilValue {
    param(
        [string]$Name,
        [scriptblock]$Check,
        [scriptblock]$IsReady,
        [int]$TimeoutSeconds = 120
    )

    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    do {
        $value = & $Check
        if (& $IsReady $value) {
            Write-Host "OK: $Name"
            return $value
        }
        if ((Get-Date) -ge $deadline) {
            throw "Timed out waiting for $Name. Last value: $value"
        }
        Start-Sleep -Seconds 2
    } while ($true)
}

$suffix = Get-Date -Format "yyyyMMddHHmmss"
$email = "e2e-$suffix@example.com"
$tag = "e2e-$suffix"
$eventID = [guid]::NewGuid().ToString()

Write-Step "subscribing $email to $Repo"
$body = @{ email = $email; repo = $Repo } | ConvertTo-Json -Compress
$headers = @{ "X-API-Key" = $ApiKey }
$response = Invoke-WebRequest -Method Post -Uri "$BaseUrl/api/subscribe" -Headers $headers -ContentType "application/json" -Body $body -UseBasicParsing -TimeoutSec 15
if ($response.StatusCode -ne 200) {
    throw "subscribe returned $($response.StatusCode)"
}
Write-Host "OK: subscribe"

Write-Step "reading confirmation token from subscription-api database"
$token = Wait-UntilValue "confirmation token" {
    $raw = docker compose -f $ComposeFile exec -T postgres-api psql -U postgres -d subber_api -t -A -c "SELECT token FROM subscriptions WHERE email='$email' AND repo='$Repo' LIMIT 1;"
    First-NonEmptyLine $raw
} {
    param($value)
    $value -ne ""
} $TimeoutSeconds

Write-Step "confirming subscription"
$response = Invoke-WebRequest -Method Get -Uri "$BaseUrl/api/confirm/$token" -UseBasicParsing -TimeoutSec 15
if ($response.StatusCode -ne 200) {
    throw "confirm returned $($response.StatusCode)"
}
Write-Host "OK: confirm"

Write-Step "waiting for scanner watchlist update through Kafka"
Wait-UntilValue "scanner watchlist row" {
    $raw = docker compose -f $ComposeFile exec -T postgres-scanner psql -U postgres -d subber_scanner -t -A -c "SELECT COUNT(*) FROM scanner_watchlist WHERE repo='$Repo';"
    [int](First-NonEmptyLine $raw)
} {
    param($value)
    $value -gt 0
} $TimeoutSeconds | Out-Null

Write-Step "publishing ReleaseDetected to Kafka"
$occurredAt = (Get-Date).ToUniversalTime().ToString("o")
$event = @{
    event_id = $eventID
    event_type = "ReleaseDetected"
    occurred_at = $occurredAt
    source = "scripted-kafka-e2e"
    correlation_id = $eventID
    payload = @{
        repo = $Repo
        tag = $tag
        url = "https://github.com/$Repo/releases/tag/$tag"
    }
} | ConvertTo-Json -Depth 5 -Compress
"$Repo|$event" | docker compose -f $ComposeFile exec -T kafka /opt/kafka/bin/kafka-console-producer.sh --bootstrap-server localhost:9092 --topic subber.release.events --property parse.key=true --property key.separator='|'
Write-Host "OK: release event published"

Write-Step "waiting for notification-service delivery"
$status = Wait-UntilValue "release notification sent" {
    $raw = docker compose -f $ComposeFile exec -T postgres-notifier psql -U postgres -d subber_notifier -t -A -c "SELECT status FROM notification_deliveries WHERE recipient_email='$email' AND repo='$Repo' AND tag='$tag' ORDER BY updated_at DESC LIMIT 1;"
    First-NonEmptyLine $raw
} {
    param($value)
    $value -eq "sent"
} $TimeoutSeconds

if ($status -ne "sent") {
    throw "notification status is $status"
}

Write-Step "checking Mailpit received messages"
$messages = Invoke-RestMethod -Uri "http://localhost:8025/api/v1/messages" -TimeoutSec 15
Write-Host "OK: Mailpit total messages = $($messages.total)"

Write-Host "Kafka E2E OK: email=$email repo=$Repo tag=$tag"
