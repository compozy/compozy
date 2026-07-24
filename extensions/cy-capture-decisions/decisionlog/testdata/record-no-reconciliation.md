---
id: AD-009
title: Structured logging with slog
status: proven
tags: [logging]
source_slug: feat-observability
source_adr: adrs/adr-005.md
promoted_at: 2026-07-11
supersedes: []
superseded_by: null
evidence: "diff 345def; verify passed"
---

## Context

Log output was unstructured and hard to query.

## Decision

Adopt log/slog with a JSON handler.

## Alternatives

- Third-party logger (rejected: extra dependency).

## Consequences

- Call sites pass structured key/value fields.
