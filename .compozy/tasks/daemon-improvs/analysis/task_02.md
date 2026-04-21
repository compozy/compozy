---
status: pending
title: Daemon Integration Harness and Validation Lane
type: infra
complexity: high
dependencies:
  - task_01
---

# Daemon Integration Harness and Validation Lane

## Overview

This task creates the reusable real-daemon harness and integration lane required by the TechSpec before the riskiest runtime and transport changes land. It establishes deterministic daemon boot, isolated home/workspace fixtures, transport clients, artifact capture, and build-tag separation without turning the work into a test-only task.

<critical>
- ALWAYS READ the TechSpec before starting; there is no PRD for this feature
- REFERENCE the TechSpec sections "RuntimeHarness", "Testing Approach", "Test Lanes", and "Development Sequencing" instead of duplicating them here
- FOCUS ON "WHAT" - create a stable execution harness and validation lane that later tasks can reuse directly
- MINIMIZE CODE - centralize daemon boot and artifact capture in one test utility package instead of cloning setup logic across suites
- TESTS REQUIRED - unit and integration coverage are mandatory for harness behavior and lane separation
</critical>

<requirements>
1. MUST create a reusable `internal/testutil/e2e` harness that boots a real daemon with isolated `$HOME`, workspace roots, HTTP and UDS clients, and deterministic cleanup.
2. MUST expose artifact capture sufficient for later tasks to inspect daemon logs, run directories, and transport outputs from integration tests.
3. MUST introduce an integration-lane strategy, including build tags and Makefile support, that keeps `make verify` focused on the fast unit/race loop while still enabling real-daemon suites.
4. MUST provide stable hooks for later CLI parity and ACP fault-injection tests without requiring each suite to recreate boot logic.
5. SHOULD make timeout and cleanup defaults deterministic enough to avoid flaky repeated daemon start/stop cycles in CI.
</requirements>

## Subtasks

- [ ] 2.1 Create `internal/testutil/e2e` helpers for isolated home layout, workspace fixtures, daemon boot, and teardown.
- [ ] 2.2 Add shared HTTP, UDS, and CLI client accessors plus artifact manifest helpers to the harness.
- [ ] 2.3 Introduce build tags and Makefile support for a dedicated integration lane that does not slow the default verification loop.
- [ ] 2.4 Refactor at least one existing daemon integration surface to use the shared harness instead of bespoke boot wiring.
- [ ] 2.5 Add tests that prove repeated harness start/stop and artifact collection are deterministic across runs.

## Implementation Details

Implement the infrastructure described in the TechSpec sections "RuntimeHarness", "Testing Approach", "Test Lanes", and "Build Order". This task should build the reusable execution foundation for later parity and fault-injection work, but it should not yet perform the full contract migration or runtime supervision hardening.

### Relevant Files

- `Makefile` - new integration-lane targets and build-tag aware test entrypoints must be wired here.
- `internal/daemon/boot_integration_test.go` - existing daemon integration behavior should be migrated toward the shared harness.
- `internal/api/httpapi/transport_integration_test.go` - current transport integration coverage is a direct consumer of the reusable daemon harness.
- `internal/cli/daemon_exec_test_helpers_test.go` - CLI integration helpers should stop inventing parallel daemon setup logic.
- `internal/core/run/executor/execution_acp_integration_test.go` - later ACP fault-injection coverage depends on the reusable daemon harness.
- `internal/testutil/e2e/harness.go` - new shared daemon boot and lifecycle harness entrypoint.
- `internal/testutil/e2e/artifacts.go` - new artifact manifest and artifact collection helpers for integration runs.

### Dependent Files

- `internal/testutil/acpmock/driver.go` - later ACP mock infrastructure should plug into the same harness rather than building a separate daemon boot layer.
- `pkg/compozy/runs/integration_test.go` - public run-reader integration coverage should reuse the shared harness clients and fixtures.
- `internal/cli/operator_commands_integration_test.go` - operator and daemon CLI parity tests should reuse the same home/workspace boot logic.
- `internal/api/client/reviews_exec_test.go` - transport-aware client tests will later benefit from the shared harness and artifact collection.

### Related ADRs

- [ADR-003: Validation-First Daemon Hardening](adrs/adr-003.md) - requires the reusable daemon harness and integration lane in the primary scope.

## Deliverables

- New `internal/testutil/e2e` package with daemon boot, isolated home/workspace fixtures, and artifact manifest helpers.
- Shared HTTP, UDS, and CLI harness clients for later parity tests.
- Integration build tags and Makefile target(s) for the real-daemon suite.
- Unit tests with 80%+ coverage for harness configuration, path setup, and artifact manifest behavior **(REQUIRED)**
- Integration tests proving deterministic daemon start/stop and client connectivity through the shared harness **(REQUIRED)**

## Tests

- Unit tests:
  - [ ] Building a harness with an isolated `$HOME` returns stable daemon, log, database, and run artifact paths without leaking caller environment state.
  - [ ] Artifact manifest generation returns the expected daemon log, run directory, and socket or HTTP metadata for a started harness.
  - [ ] Harness cleanup closes transport clients and removes temporary state without leaving stale daemon info or sockets behind.
- Integration tests:
  - [ ] Starting a harnessed daemon and probing `GET /api/daemon/status` succeeds over both HTTP and UDS using the shared clients.
  - [ ] Repeated harness start/stop cycles do not leave stale singleton artifacts and can be rerun in the same CI process without flaking.
  - [ ] A CLI command executed through the harness can locate the same daemon instance and record artifacts in the shared manifest.
- Test coverage target: >=80%
- All tests must pass

## Success Criteria

- All tests passing
- Test coverage >=80%
- Real-daemon integration tests share one reusable harness instead of bespoke setup code
- Integration-lane entrypoints are explicit and do not bloat `make verify`
- Later transport, CLI, and ACP suites can reuse the same harness and artifact manifest
