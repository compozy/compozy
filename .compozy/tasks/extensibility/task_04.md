---
status: completed
title: Capability enforcement and audit log
type: backend
complexity: medium
dependencies:
  - task_02
---

# Task 04: Capability enforcement and audit log

## Overview
Implement the capability enforcement layer and the per-run audit log writer. The enforcement layer checks every operation (hook dispatch and Host API call) against the capabilities an extension accepted during its initialize handshake, returning `-32001 capability_denied` when the grant is missing. The audit log appends one JSONL record per call to `.compozy/runs/<run-id>/extensions.jsonl` so operators can retroactively inspect what each extension did during a run.

<critical>
- ALWAYS READ the PRD and TechSpec before starting
- REFERENCE TECHSPEC for implementation details — do not duplicate here
- FOCUS ON "WHAT" — describe what needs to be accomplished, not how
- MINIMIZE CODE — show code only to illustrate current structure or problem areas
- TESTS REQUIRED — every task MUST include tests in deliverables
- NOTE: No `_prd.md` exists. Requirements derive from `_techspec.md`, `_protocol.md`, and ADR-005.
</critical>

<requirements>
- MUST implement a `CapabilityChecker` that, given an extension's accepted capabilities and a requested method or hook event, returns nil for allowed calls and a structured `CapabilityDeniedError` carrying `missing` and `granted` lists for denied calls.
- MUST support method-to-capability mapping from the Host API inventory in `_protocol.md` section 5.2 and hook-event-to-capability mapping from ADR-005.
- MUST implement an `AuditLogger` that appends one JSONL record per hook dispatch or Host API call to `.compozy/runs/<run-id>/extensions.jsonl`.
- MUST include in each audit record: timestamp, extension name, direction (`host→ext` or `ext→host`), method, capability exercised, latency in milliseconds, result (`ok` or `error`), and error detail when applicable.
- MUST make the audit writer safe for concurrent use across goroutines.
- MUST flush the audit log on run shutdown and recover cleanly if the process exits mid-write (best-effort append, no locking that would deadlock on crash).
- MUST NOT log secret material or any payload body; only method-level metadata.
- MUST emit a standard `log/slog` warning when a capability denial happens so the CLI can surface it without reading the audit log.
</requirements>

## Subtasks
- [x] 04.1 Implement `CapabilityChecker` with its method-to-capability and hook-to-capability lookup tables derived from ADR-005.
- [x] 04.2 Implement `CapabilityDeniedError` as a typed error that formats a `-32001 capability_denied` JSON-RPC error payload when serialized.
- [x] 04.3 Implement `AuditLogger.Open(runArtifactsPath)` that creates or truncates `extensions.jsonl` under the run artifact root.
- [x] 04.4 Implement `AuditLogger.Record(entry)` that marshals to a single JSON line and appends atomically, safe for concurrent writers.
- [x] 04.5 Implement `AuditLogger.Close(ctx)` with deadline-aware flush and fsync semantics.
- [x] 04.6 Write table-driven tests for the checker (allowed, denied single, denied multi) and integration tests for the audit writer (concurrent writes, crash-safe append).

## Implementation Details
See TechSpec "Implementation Design → Data Models" for the audit entry schema, "Monitoring and Observability" for how the audit log relates to the event bus, `_protocol.md` section 5.2 for the method-to-capability mapping, and ADR-005 for the full capability taxonomy.

Place files under:
- `internal/core/extension/capability.go` — checker, error types, lookup tables
- `internal/core/extension/audit.go` — audit logger
- `internal/core/extension/capability_test.go`
- `internal/core/extension/audit_test.go`

Key invariants:
- The checker is a pure function of the accepted-capabilities set plus the method/hook name. No I/O.
- The audit logger is the only component that writes to `extensions.jsonl`. Other components call it through a handler interface for testability.
- The audit writer must not block hook dispatch for more than a few milliseconds under normal conditions. Use a bounded buffered writer if needed.

### Relevant Files
- `internal/core/extension/manifest.go` — Source of the capability taxonomy and `SecurityConfig.Capabilities`.
- `internal/core/run/journal/journal.go` — Precedent for per-run append-only writers.
- `pkg/compozy/events/event.go` — Existing `log/slog` usage patterns.
- `_protocol.md` section 5.2 — Host API method to capability mapping.
- `adrs/adr-005.md` — Capability taxonomy reference.

### Dependent Files
- Task 05 (dispatcher) calls `CapabilityChecker` before every hook dispatch and logs each call through `AuditLogger`.
- Task 06 (Host API services) calls `CapabilityChecker` on entry for every Host API method.
- Task 07 (run-scope bootstrap) constructs and owns the `AuditLogger` for the run.

### Related ADRs
- [ADR-005: Capability-Based Security Without Trust Tiers](adrs/adr-005.md) — Capability model, enforcement points, and audit expectations.

## Deliverables
- `internal/core/extension/capability.go` with checker, error type, and lookup tables.
- `internal/core/extension/audit.go` with concurrent-safe JSONL writer and lifecycle methods.
- Unit tests with 80%+ coverage **(REQUIRED)**
- Integration tests for audit writer under concurrent load **(REQUIRED)**

## Tests
- Unit tests:
  - [x] Checker returns nil when the accepted set contains the exact capability a Host API method requires.
  - [x] Checker returns `CapabilityDeniedError` with `missing = [tasks.create]` when the set omits the required grant.
  - [x] Checker maps hook events to capabilities consistent with ADR-005 (e.g., `prompt.post_build` requires `prompt.mutate`).
  - [x] `CapabilityDeniedError` serializes to a valid `-32001` JSON-RPC error object with structured `data`.
  - [x] Audit writer rejects opening a run artifact directory that does not exist.
  - [x] Audit writer round-trips a recorded entry through JSONL parse.
  - [x] `AuditLogger.Close` flushes pending entries before returning.
- Integration tests:
  - [x] 100 goroutines concurrently calling `AuditLogger.Record` produce 100 valid JSONL records with no interleaved lines.
  - [x] Killing the writer mid-operation leaves a readable prefix on disk (no torn records).
- Test coverage target: >=80%
- All tests must pass

## Success Criteria
- All tests passing
- Test coverage >=80%
- `make verify` exits zero with zero lint issues
- Every capability denial path in downstream tasks routes through `CapabilityChecker` without duplicating logic
- Every hook dispatch and Host API call in downstream tasks produces exactly one audit record
