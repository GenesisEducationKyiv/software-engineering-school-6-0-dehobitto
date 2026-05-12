# Software Design Document — Subber

## 1. Introduction

**Purpose**: Define the architecture and design of Subber.

**Scope**: REST API, API key authentication, background worker architecture, PostgreSQL storage, Redis caching, and Docker deployment.

**Architecture**: Single monolith with two concurrent background workers. No microservices.

---

## 2. System Overview

![System Context Diagram](images/sdd-Context.drawio.png)

1. A user submits their email and a GitHub repository via the web form
2. Subber saves the subscription as unconfirmed and sends a confirmation email
3. The user clicks the confirmation link — the subscription becomes active
4. A background scanner periodically polls GitHub for new release tags
5. When a new tag is detected, every confirmed subscriber of that repository receives an email

---

## 3. Architecture

### Technology Stack:

| Layer | Technology |
|---|---|
| Language | Go |
| HTTP framework | Gin |
| Database | PostgreSQL |
| Cache | Redis |
| Email | SMTP |
| Containerisation | Docker, Docker Compose |

### System Containers
![Container Diagram](images/sdd-Container.drawio.png)

**Components**:
- **Subber App (Go)** - handles API requests, polling, and email delivery
- **PostgreSQL** - stores all subscription state
- **Redis** - caches GitHub API responses to reduce external calls

### Backend Components
![Component Diagram](images/sdd-Component.drawio.png)

Internally, the Go monolith is divided into:
- **Subscribe API** - handles HTTP requests and DB writes
- **Scanner Worker** - periodically checks GitHub for new release tags
- **Notifier Worker** - consumes jobs from an internal channel and delivers emails via SMTP

**Data flow**:

1. Subscription flow:

```mermaid
sequenceDiagram
    participant User
    participant UI as Web UI
    participant API as REST API
    participant GitHub as GitHub API
    participant DB as PostgreSQL
    participant Notifier as Notifier Worker
    participant SMTP as SMTP Server

    User->>UI: fills in email and repo
    UI->>API: POST /api/subscribe
    API->>GitHub: does this repo exist?
    GitHub-->>API: 200 OK
    API->>DB: save subscription (unconfirmed)
    API->>Notifier: enqueue confirmation job
    Notifier->>SMTP: send confirmation email
    SMTP-->>User: email with confirmation link

    User->>API: GET /api/confirm/:token
    API->>DB: mark confirmed = true
    API-->>User: confirmed
```

2. Release notification flow:

```mermaid
sequenceDiagram
    participant Scanner as Scanner Worker
    participant DB as PostgreSQL
    participant GitHub as GitHub API
    participant Notifier as Notifier Worker
    participant SMTP as SMTP Server
    participant User

    loop every 30 seconds
        Scanner->>DB: get all confirmed subscriptions
        DB-->>Scanner: list of repos + last seen tags
        Scanner->>GitHub: get latest release tag
        GitHub-->>Scanner: tag_name
        alt tag changed
            Scanner->>DB: update last_seen_tag
            Scanner->>Notifier: enqueue notification job
            Notifier->>SMTP: send release email
            SMTP-->>User: new release notification
        end
    end
```

---

## 4. Non-Functional Properties

| Property | Value      | Rationale |
|---|------------|---|
| Scan interval | 30 seconds | Balances notification latency against GitHub API rate limits |
| GitHub API cache TTL | 45 seconds | Reduces repeated calls for repos with no new releases |
| GitHub HTTP client timeout | 10 seconds | Prevents scanner from stalling on slow API responses |
| HTTP server read header timeout | 10 seconds | Mitigates Slowloris-style connection exhaustion |
| Scan cycle timeout | 20 seconds | Bounds the worst-case duration of a single scan pass |

---

## 5. Data Model
**Relations**: Single denormalized table. No separate `users` or `repositories` tables to eliminate `JOIN` overhead.

**Table**: `subscriptions`

| Field | Type | Purpose |
|---|---|---|
| `email` | String | Subscriber's email address (part of PK) |
| `repo` | String | Target repository, e.g., `owner/repo` (part of PK) |
| `confirmed` | Boolean | Guards against unverified addresses receiving notifications |
| `token` | UUID | Used in confirmation and unsubscribe links |
| `last_seen_tag` | String | Baseline for detecting new releases |

**Search Patterns**:
- **User view**: Fetch all subscriptions for a specific `email`.
- **Auth**: Look up a single record by `token` (confirm/unsubscribe).
- **Scanner target list**: Fetch distinct `repo`s where `confirmed = true`.
- **Notifier dispatch**: Fetch all `email`s for a specific `repo` where `confirmed = true`.

**Index Strategy**:
- **Composite Primary Key** (`email`, `repo`): Prevents duplicates and implicitly indexes `email` for the user view.
- **Unique Index** (`token`): Ensures fast O(1) lookups for confirmation links.
- **Partial Index** (`repo`) `WHERE confirmed = true`: Keeps index size minimal while highly optimizing Scanner and Notifier worker queries.

Schema is embedded into the binary and applied on startup.

---

## 6. External Interfaces

**GitHub API** - two call types:

- *Repository existence check* (on subscribe): HEAD request to `/repos/{owner}/{repo}`. 404 → subscription rejected with an error. Other non-200 → propagated as an error to the caller.
- *Latest release tag* (polled by Scanner): GET `/repos/{owner}/{repo}/releases/latest`. 404 → no release yet, skip silently. 429 → rate limit exceeded, error returned and scan skipped for that repo. Other non-200 → error returned and scan skipped. Successful responses are cached in Redis for 10 minutes to reduce API call volume.

**SMTP** - sends two email types: subscription confirmation and release notification.

**Web Interface** - static form at `/` for submitting email and repository.

---

## 7. API

Protected endpoints require `X-API-Key` header.

| Method | Path | Auth | Description |
|---|---|---|---|
| `POST` | `/api/subscribe` | ✓ | Subscribe email to a repository |
| `GET` | `/api/subscriptions/` | ✓ | List confirmed subscriptions for an email |
| `GET` | `/api/confirm/:token` | — | Confirm a subscription |
| `GET` | `/api/unsubscribe/:token` | — | Remove a subscription |
| `GET` | `/metrics` | — | Prometheus metrics |

---

## 8. Security

- **Authentication**: API key via `X-API-Key` header on protected routes
- **Email ownership**: double opt-in - confirmation required before any notifications are sent
- **Tokens**: one UUID per subscription for confirmation and unsubscribe
- **Credentials**: all secrets injected via environment variables

---

## 9. Deployment

Three Docker Compose services: `postgres`, `redis`, `subber`. The application waits for both dependencies to be healthy before starting.

**CI** (GitHub Actions): build, test, and lint run on every push and pull request to `main`.
