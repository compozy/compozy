---
status: pending
title: Regression Coverage, Docs, and Migration Cleanup
type: chore
complexity: medium
dependencies:
  - task_12
  - task_13
  - task_14
  - task_15
---

# Regression Coverage, Docs, and Migration Cleanup

## Overview
This task closes the daemon migration by hardening end-to-end coverage, updating the user-facing documentation, and removing stale references to the pre-daemon model. It is the cleanup pass that turns the implementation from a working migration into a maintainable, documented product surface.

<critical>
- ALWAYS READ the TechSpec before starting; there is no PRD for this feature
- REFERENCE the TechSpec sections "Testing Approach", "CLI/TUI clients", and "Known Risks" instead of duplicating them here
- FOCUS ON "WHAT" — close the migration with real regression evidence and clear docs, not just partial cleanup
- MINIMIZE CODE — prefer deleting stale references and extending existing regression suites over inventing new test harnesses unless coverage truly requires it
- TESTS REQUIRED — unit and integration coverage are mandatory, and `make verify` is a hard gate
</critical>

<requirements>
1. MUST add or update regression coverage for the daemon, CLI, TUI, sync, archive, reviews, exec, and public run-reader paths changed by this migration.
2. MUST update user-facing docs to reflect the new daemon commands, attach/watch semantics, transport model, and home-scoped runtime layout.
3. MUST remove or migrate stale references to legacy `_tasks.md`, `_meta.md`, and `compozy start` behavior.
4. MUST finish with a clean `make verify` run as the final migration gate.
5. SHOULD keep TechSpec, ADR, and task references internally consistent with the final implementation surface.
6. MUST remove temporary compatibility shims and deprecated migration leftovers instead of normalizing them into the final design.
</requirements>

## Subtasks
- [ ] 16.1 Extend regression suites for the daemon-backed CLI, TUI, sync, archive, review, exec, and public-reader flows.
- [ ] 16.2 Update README and project docs for the daemon runtime, commands, and operational model.
- [ ] 16.3 Remove stale migration leftovers and obsolete references to generated metadata and legacy start behavior.
- [ ] 16.4 Verify all golden/help/test artifacts still match the final command surface.
- [ ] 16.5 Run the full repository verification gate and fix any remaining regressions before closing the feature.

## Implementation Details
Implement the closeout described in the TechSpec "Testing Approach", "CLI/TUI clients", and "Known Risks" sections. This task should focus on evidence and cleanup: broaden the changed-surface regression net, update the operator documentation, and eliminate stale references that would otherwise keep the codebase half on the old model.

### Relevant Files
- `README.md` — top-level user-facing documentation that must reflect the daemon runtime and command surface.
- `docs/extensibility/architecture.md` — extension and runtime docs that must reflect daemon-managed execution ownership.
- `internal/cli/root_command_execution_test.go` — broad CLI regression suite for the new command surface.
- `pkg/compozy/runs/integration_test.go` — public run-reader integration coverage that must prove daemon-backed behavior end to end.
- `internal/core/migration/migrate_test.go` — migration cleanup coverage for legacy metadata and workflow transitions.
- `internal/core/run/executor/execution_acp_integration_test.go` — execution integration coverage that should protect daemon-backed runtime behavior.

### Dependent Files
- `internal/cli/testdata/start_help.golden` — stale golden data must be removed or replaced to match the final CLI.
- `internal/cli/testdata/exec_help.golden` — help output must remain aligned with the daemon-backed exec surface.
- `.compozy/tasks/daemon/_techspec.md` — final implementation and docs should stay aligned with the approved technical design.
- `.compozy/tasks/daemon/_tasks.md` — task tracking should remain consistent with the actual migration closeout.

### Related ADRs
- [ADR-003: Expose the Daemon Through AGH-Aligned REST Transports Using Gin](adrs/adr-003.md) — docs and tests must reflect the finalized transport contract.
- [ADR-004: Preserve TUI-First UX While Introducing Auto-Start and Explicit Workspace Operations](adrs/adr-004.md) — final UX and docs should match the approved direction.

## Deliverables
- Expanded regression coverage for the daemon migration's highest-risk surfaces.
- Updated user-facing documentation for commands, runtime layout, and attach/watch behavior.
- Cleanup of stale references to legacy metadata files and removed commands.
- Unit tests with 80%+ coverage for changed cleanup surfaces **(REQUIRED)**
- Integration tests plus a passing `make verify` run **(REQUIRED)**

## Tests
- Unit tests:
  - [ ] CLI and migration regression suites cover the final daemon-backed command surface without references to the removed legacy start path.
  - [ ] Golden/help fixtures match the final command output and flag set.
  - [ ] Legacy metadata cleanup logic keeps `_tasks.md` and `_meta.md` out of the final operational flow.
- Integration tests:
  - [ ] End-to-end daemon-backed runs, attach/watch, sync/archive, and exec flows remain green in the repository integration suites.
  - [ ] Public `pkg/compozy/runs` integration tests prove daemon-backed run inspection end to end.
  - [ ] `make verify` passes cleanly after docs, cleanup, and regression updates.
- Test coverage target: >=80%
- All tests must pass

## Success Criteria
- All tests passing
- Test coverage >=80%
- Documentation reflects the final daemon runtime and command model accurately
- Stale references to the legacy execution model are removed
- The full repository verification gate passes at the end of the migration
