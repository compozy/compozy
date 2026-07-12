---
id: AD-001
title: Event-sourcing for orders
status: proven
tags: [orders, async]
source_slug: feat-orders
source_adr: adrs/adr-002.md
promoted_at: 2026-07-11
supersedes: []
superseded_by: null
evidence: "cy-final-verify report p99<200ms; diff abc123; issue_003 resolved"
---

## Context

Orders needed an auditable, replayable write model.

## Decision

Adopt event-sourcing for the orders aggregate.

## Alternatives

- CRUD with an audit table (rejected: replay is lossy).

## Consequences

- Read models must be projected from the event stream.

## Reconciliation

Implemented as designed. Evidence: diff abc123; verify p99<200ms.
