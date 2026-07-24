---
id: AD-005
title: Standardize on wrapped errors
status: candidate
tags: [errors]
source_slug: feat-observability
source_adr: adrs/adr-004.md
promoted_at: 2026-07-11
supersedes: []
superseded_by: null
evidence: ""
---

## Context

Error handling was inconsistent across packages.

## Decision

Wrap all errors with fmt.Errorf and %w.

## Alternatives

- Sentinel-only errors (rejected: loses call-site context).

## Consequences

- Callers can use errors.Is/As reliably.

## Reconciliation

Relevance-gated but not visible in this feature's diff; no evidence yet, so recorded as candidate.
