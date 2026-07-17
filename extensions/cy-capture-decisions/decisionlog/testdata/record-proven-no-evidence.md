---
id: AD-006
title: Standardize on wrapped errors
status: proven
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

Marked proven but evidence is empty — this record must fail validation.
