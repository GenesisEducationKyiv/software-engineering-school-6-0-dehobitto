# ADR-001: Select caching tool for GitHub API responses
**Status:** Pending (waiting for a review)
**Date:** 2026-05-08
**Author:** Oleksandr Makarov

## Context
- The scanner worker polls GitHub API every 30 seconds for every confirmed subscription - one request per subscription per cycle.
- GitHub's unauthenticated rate limit is 60 requests/hour - even a small number of subscriptions exhausts this quickly. Authenticated requests raise the limit to 5000/hour, but caching reduces pressure regardless and avoids redundant calls when no new release exists.
- We need a cache that supports TTL expiry so stale tags are not served indefinitely.

## Variants considered
**1. Redis**
- **Positives:** TTL is a native primitive, survives process restarts, cache-aside fallback is trivial to implement.
- **Negatives:** external dependency, additional service to operate.

**2. In-memory map (`sync.Map`)**
- **Positives:** zero dependencies, no operational overhead
- **Negatives:** cache lost on every restart causing a burst of API calls on startup, requires a manual eviction goroutine for TTL.

## Final choice
**Redis selected.**

The decisive factor is restart behavior - an in-memory cache is lost on every process restart, causing a burst of GitHub API calls proportional to the number of subscriptions. Redis TTL support also eliminates the need for a custom eviction loop.

## Implementation Details

* **Key:** `github:latest_tag:{owner/repo}`
* **Value:** latest release tag string (e.g. `v1.2.3`)
* **TTL:** 10 minutes
* **Pattern:** cache-aside - check cache before every GitHub call, write on cache miss. If Redis is unavailable, the call falls through to GitHub silently with no error returned to the user.

## Consequences
### Positives:
- GitHub API call volume reduced significantly under real subscription load
- TTL expiry is handled by Redis natively — no application-level eviction code
- Cache survives process restarts

### Negatives:
- More complicated Docker setup
- One more failure point in the infrastructure, though the fallback means Redis downtime degrades performance rather than causing an outage
