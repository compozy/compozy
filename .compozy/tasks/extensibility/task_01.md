---
status: completed
title: Extract internal/core/subprocess package and add protocol version constant
type: refactor
complexity: medium
dependencies: []
---

# Task 01: Extract internal/core/subprocess package and add protocol version constant

## Overview
Extract the JSON-RPC subprocess lifecycle code (spawn, transport, handshake, signal handling, graceful shutdown) from `internal/core/agent` into a new shared package `internal/core/subprocess` so both the existing ACP agent client and the upcoming extension manager can consume the same primitives. Also add the `ExtensionProtocolVersion = "1"` constant to `internal/version` so every consumer speaks the same wire version.

<critical>
- ALWAYS READ the PRD and TechSpec before starting
- REFERENCE TECHSPEC for implementation details — do not duplicate here
- FOCUS ON "WHAT" — describe what needs to be accomplished, not how
- MINIMIZE CODE — show code only to illustrate current structure or problem areas
- TESTS REQUIRED — every task MUST include tests in deliverables
- NOTE: No `_prd.md` exists for this feature. Technical requirements are derived from `_techspec.md` and `_protocol.md`.
</critical>

<requirements>
- MUST create a new Go package at `internal/core/subprocess` that exposes a protocol-agnostic process lifetime API (spawn, wait, kill, line-framed JSON-RPC transport, initialize handshake, signal escalation).
- MUST move the Unix process configuration from `internal/core/agent/process_unix.go` and any equivalent Windows handling into the new package without changing ACP behavior.
- MUST update `internal/core/agent` consumers to import the new package instead of the internal files that were moved.
- MUST NOT introduce any new abstractions during extraction — the first pass is verbatim file relocation plus import adjustments.
- MUST add `ExtensionProtocolVersion = "1"` as an exported constant in `internal/version` so both ACP (informational) and the extension manager can reference it.
- MUST keep every existing test in `internal/core/agent` passing without modification beyond import path updates.
- MUST NOT break `make verify` (fmt + lint + test + build) at the end of the task.
</requirements>

## Subtasks
- [x] 01.1 Create `internal/core/subprocess` package directory and move `process_unix.go` (and any pre-existing platform variants) into it verbatim, preserving `Setpgid`-based process group handling.
- [x] 01.2 Move the ACP-neutral subprocess helpers currently living in `internal/core/agent` (process launch, wait, kill, line-framed JSON-RPC transport plumbing, initialize handshake scaffolding, SIGTERM→SIGKILL escalation) into the new package, keeping exported names stable.
- [x] 01.3 Update `internal/core/agent` to import the new package and re-wire ACP-specific logic on top of the shared primitives.
- [x] 01.4 Add `ExtensionProtocolVersion` constant to `internal/version`.
- [x] 01.5 Run the existing ACP test suite to confirm no regressions, then add focused unit tests for the newly extracted package covering transport framing, handshake happy-path, and SIGTERM→SIGKILL escalation.
- [x] 01.6 Run `make verify` and ensure zero lint issues and all tests pass.

## Implementation Details
See TechSpec "System Architecture → Component Overview" for the boundary between `internal/core/subprocess` and `internal/core/extension`, and "Development Sequencing → Build Order" step 1 for the extraction sequence.

Refactor approach per ADR-003:
- First pass is verbatim movement — no new abstractions.
- Abstractions are only introduced later when both the ACP path and the extension path prove they need them.
- Windows handling: if `internal/core/agent/process_windows.go` does not exist yet, this task does **not** add Windows support. Document the gap in the task summary and leave a `// TODO(windows)` pointer in the package.

Risk: this task touches a production code path already used by ACP agents. The existing ACP test suite is the safety net — do not relax, skip, or disable any of those tests.

### Relevant Files
- `internal/core/agent/client.go` — ACP client today; will remain but will import the extracted subprocess package.
- `internal/core/agent/process_unix.go` — Unix-specific Setpgid process group setup to be moved verbatim.
- `internal/core/agent/registry_launch.go` — ACP runtime launch commands; will not move but will import the new package's launcher.
- `internal/core/agent/session.go` — ACP session abstraction; will keep ACP-specific message handling.
- `internal/version/version.go` — Destination for `ExtensionProtocolVersion` constant.

### Dependent Files
- `internal/core/agent/client_test.go` — Must continue passing with updated imports.
- Any file under `internal/core/agent` that currently references the subprocess helpers directly — imports will need to be updated to point at the new package.
- Downstream consumers of `internal/core/agent` in `internal/core/run/executor` — unaffected if public API of `internal/core/agent` stays stable.

### Related ADRs
- [ADR-001: Subprocess-Only Extension Model](adrs/adr-001.md) — The extraction is the structural prerequisite for a subprocess-based extension tier.
- [ADR-003: JSON-RPC 2.0 over stdio with Shared internal/core/subprocess Package](adrs/adr-003.md) — Primary ADR governing this task.

## Deliverables
- New `internal/core/subprocess` package containing the moved files and their tests.
- Updated `internal/core/agent` package importing the new package, with no behavioral change.
- `ExtensionProtocolVersion = "1"` added to `internal/version`.
- Unit tests with 80%+ coverage for the new package **(REQUIRED)**
- Integration tests preserved from the existing ACP suite **(REQUIRED)**

## Tests
- Unit tests:
  - [x] JSON-RPC envelope encodes and decodes a well-formed request with integer and string IDs.
  - [x] Line-framed transport rejects messages larger than the 10 MiB limit with a structured error.
  - [x] Transport ignores blank lines on the read side and never emits blank lines on the write side.
  - [x] Spawning a trivial echo binary returns a `Process` whose `Wait` observes the child exit code.
  - [x] Killing a spawned Unix process group via `Setpgid` terminates all descendants within the grace window.
  - [x] SIGTERM escalates to SIGKILL when the child does not exit before the deadline.
  - [x] Handshake returns `-32602 invalid params` when the child advertises an unsupported protocol version.
- Integration tests:
  - [x] Existing `internal/core/agent` ACP client tests still pass unchanged after the extraction.
  - [x] A minimal end-to-end launch of an ACP mock still completes initialize handshake against the new package.
- Test coverage target: >=80%
- All tests must pass

## Success Criteria
- All tests passing
- Test coverage >=80%
- `make verify` exits zero with zero lint issues
- `internal/core/agent` no longer owns process lifetime primitives; it consumes them from `internal/core/subprocess`
- `version.ExtensionProtocolVersion` is the single source of truth referenced by any future extension handshake code
