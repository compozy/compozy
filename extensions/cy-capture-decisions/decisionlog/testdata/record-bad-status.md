---
id: AD-007
title: Retry policy for webhooks
status: accepted
tags: [webhooks]
source_slug: feat-webhooks
source_adr: adrs/adr-001.md
promoted_at: 2026-07-11
supersedes: []
superseded_by: null
evidence: "diff def456; verify passed"
---

## Context

Webhook delivery needed a bounded retry policy.

## Decision

Exponential backoff capped at five attempts.

## Alternatives

- Unbounded retries (rejected: poison messages).

## Consequences

- A dead-letter path is required for exhausted retries.

## Reconciliation

Status "accepted" is not in the enum; this record must fail validation.
