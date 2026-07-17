---
id: AD-008
title: Cache invalidation on write
status: proven
tags: [cache]
source_adr: adrs/adr-003.md
promoted_at: 2026-07-11
supersedes: []
superseded_by: null
evidence: "diff 012abc; verify passed"
---

## Context

Stale cache reads followed writes under load.

## Decision

Invalidate the cache key inside the write transaction.

## Alternatives

- TTL-only expiry (rejected: stale window too large).

## Consequences

- Writes pay a small invalidation cost.

## Reconciliation

The required source_slug field is absent; this record must fail validation.
