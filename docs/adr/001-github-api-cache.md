## ADR-001: Select caching tool for GitHub API responses

**Status:** Pending (waiting for a review)
**Date:** 2026-05-13
**Author:** Oleksandr Makarov

## Context

* The scanner worker polls GitHub API every 30 seconds for every confirmed subscription - one request per subscription per cycle.
* GitHub's unauthenticated rate limit is 60 requests/hour - even a small number of subscriptions exhausts this quickly. Authenticated requests raise the limit to 5000/hour, but caching reduces pressure regardless and avoids redundant calls when no new release exists.
* We need a cache that supports TTL expiry so stale tags are not served indefinitely.
* The cache TTL must be derived from the scan interval (30s) to ensure real-time freshness;

## Variants considered

**1. Redis**

* **Positives:** TTL is a native primitive, survives process restarts, allows horizontal scaling by sharing state between multiple worker instances.
* **Negatives:** external dependency, additional service to operate.

**2. In-memory map (`sync.Map`)**

* **Positives:** zero dependencies, no operational overhead.
* **Negatives:** cache lost on every restart causing a burst of API calls on startup, requires a manual eviction goroutine for TTL, cannot be shared across multiple service instances.

**3. HTTP Conditional Requests (ETags)**

* **Positives:** 304 Not Modified responses do not count against rate limits, ensures 100% data freshness.
* **Negatives:** Still requires a network round-trip for every 30s cycle, requires persistent storage for ETag values to survive restarts.

## Final choice

**Redis selected.**

The decisive factor is the coordination of distributed state and restart resilience. Redis allows us to implement a "Short-Circuit" TTL that matches our polling frequency, ensuring we never check the same repository more than once per 30-second cycle across the entire infrastructure. Redis also prevents a "cold start" burst of API calls during deployments by persisting the last known tags and ETags.

## Implementation Details

* **Key:** `github:latest_tag:{owner/repo}`
* **Value:** JSON object containing `tag` string and `etag` header value.
* **TTL:** 45 seconds (Scan interval + 50% buffer to ensure overlap coverage).
* **Pattern:** Hybrid cache-aside. The worker checks Redis first; if the record is younger than 30s, it skips the call. If the record is missing or near expiry, it performs a conditional GitHub request using the stored ETag to preserve the rate limit.

## Consequences

### Positives:

* Real-time freshness is maintained (30s detection window) while minimizing API quota usage.
* GitHub API call volume reduced significantly via ETags even when the cache "misses."
* Cache survives process restarts, preventing startup rate-limit exhaustion.
* The system is ready for horizontal scaling (multiple workers sharing one Redis state).

### Negatives:

* More complicated Docker setup.
* One more failure point in the infrastructure, though the fallback means Redis downtime degrades performance rather than causing an outage.