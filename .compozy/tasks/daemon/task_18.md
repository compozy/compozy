---
status: pending
title: Daemon QA execution and operator-flow validation
type: test
complexity: critical
dependencies:
  - task_17
---

# Task 18: Daemon QA execution and operator-flow validation

## Overview

Execute the full QA pass for the daemon migration using the artifacts from `task_17`, and leave fresh evidence for the CLI, API, TUI, sync/archive, review, exec, and `pkg/compozy/runs` surfaces. This task is responsible for turning the daemon QA plan into real validation, fixing root-cause regressions when they appear, and publishing a verification report backed by current repository evidence.

<critical>
- ALWAYS READ `_techspec.md`, ADRs, and the QA artifacts from `task_17` before running any validation
- ACTIVATE `/qa-execution` with `qa-output-path=.compozy/tasks/daemon` before any live verification or evidence capture
- IF QA FINDS A BUG, ACTIVATE `/systematic-debugging` AND `/no-workarounds` BEFORE CHANGING CODE OR TESTS
- FOLLOW THE REPOSITORY QA CONTRACT — use the discovered verify/build/test commands, real CLI and API entrypoints, and existing integration harnesses instead of one-off scripts
- DO NOT INVENT A BROWSER LANE — if no daemon web surface exists on the execution branch, report browser validation as blocked or out of scope explicitly
- DO NOT WEAKEN TESTS TO GET GREEN — fix production code or configuration at the source, then rerun the affected scenarios and full gates
- GREENFIELD: the daemon migration is only complete when the normal repo verification gates and the daemon-specific execution matrix both pass with fresh evidence
</critical>

<requirements>
1. MUST use the `/qa-execution` skill with `qa-output-path=.compozy/tasks/daemon`.
2. MUST consume `.compozy/tasks/daemon/qa/test-plans/` and `.compozy/tasks/daemon/qa/test-cases/` from `task_17` as the execution matrix seed.
3. MUST validate daemon bootstrap/recovery, workspace registry flows, task runs, review runs, exec runs, sync/archive behavior, attach/watch behavior, UDS/HTTP parity, and `pkg/compozy/runs` compatibility using real repository entrypoints.
4. MUST write fresh QA evidence to `.compozy/tasks/daemon/qa/verification-report.md` and capture bugs/screenshots/logs under the same artifact root when relevant.
5. MUST add or update regression coverage for any daemon regression discovered during execution using the narrowest durable test surface that matches the failing behavior.
6. MUST rerun the repository verification gates after the last fix, including `make verify`, and include the executed commands in the final verification report.
7. SHOULD record browser or TUI flows as `Blocked` or `Manual-only` only when the repository truly lacks a stable automation harness for that surface.
</requirements>

## Subtasks
- [ ] 18.1 Activate `/qa-execution` with `qa-output-path=.compozy/tasks/daemon` and derive the execution matrix from `task_17` artifacts
- [ ] 18.2 Discover the repository QA contract, establish the daemon baseline, and document any existing blockers before scenario testing
- [ ] 18.3 Execute CLI, API, transport-parity, sync/archive, review, exec, attach/watch, and `pkg/compozy/runs` scenarios with durable evidence capture
- [ ] 18.4 Fix root-cause regressions with matching regression coverage and rerun the impacted daemon scenarios
- [ ] 18.5 Rerun `make verify`, finalize `.compozy/tasks/daemon/qa/verification-report.md`, and publish any bug artifacts or blockers

## Implementation Details

See TechSpec sections "Testing Approach", "Known Risks", "Development Sequencing", "CLI/TUI clients", and "Transport Contract". The key constraint is that daemon QA must rely on the real repo verification contract and the daemon’s real operator surfaces, not exploratory one-off checks that cannot be repeated later.

### Relevant Files
- `.agents/skills/qa-execution/SKILL.md` — required workflow for repository-contract discovery, evidence capture, and verification reporting
- `.agents/skills/qa-execution/scripts/discover-project-contract.py` — canonical repo-contract discovery entrypoint required by `/qa-execution`
- `.agents/skills/qa-execution/references/checklist.md` — required execution-scope checklist for QA completeness
- `.agents/skills/qa-execution/references/e2e-coverage.md` — required source for deciding whether a harness exists for daemon-critical flows
- `.agents/skills/qa-execution/assets/verification-report-template.md` — required verification report structure
- `.agents/skills/qa-execution/assets/issue-template.md` — required bug report template shared with `qa-report`
- `.compozy/tasks/daemon/qa/test-plans/` — planning artifacts that seed the execution matrix
- `.compozy/tasks/daemon/qa/test-cases/` — test cases and automation annotations that the execution pass must honor
- `Makefile` — repository-defined verification gate that must pass before completion
- `internal/cli/root_command_execution_test.go` — broad CLI regression suite for daemon-backed command behavior
- `internal/core/run/executor/execution_ui_test.go` — integration seam for attach/watch and UI-oriented daemon execution behavior
- `internal/core/migration/migrate_test.go` — regression seam for sync/archive cleanup and metadata-removal behavior
- `pkg/compozy/runs/integration_test.go` — public run-reader integration coverage that must prove daemon-backed behavior end to end

### Dependent Files
- `.compozy/tasks/daemon/qa/verification-report.md` — final QA evidence written by `/qa-execution`
- `.compozy/tasks/daemon/qa/screenshots/` — screenshot evidence when a visual or TUI-related artifact is captured
- `.compozy/tasks/daemon/qa/issues/BUG-*.md` — structured bug reports for failures discovered during execution
- `.compozy/tasks/daemon/qa/test-cases/*.md` — may be updated if execution reveals missing or blocked annotations
- `internal/cli/*_test.go` — may gain daemon-regression coverage if execution finds CLI issues
- `internal/core/run/executor/*_test.go` — may gain regression coverage for attach/watch or execution lifecycle issues
- `pkg/compozy/runs/*_test.go` — may gain regression coverage for public run-reader issues

### Related ADRs
- [ADR-001: Adopt a Global Home-Scoped Singleton Daemon](adrs/adr-001.md) — execution must validate singleton boot, recovery, and multi-workspace safety
- [ADR-002: Keep Human Artifacts in the Workspace and Move Operational State to Home-Scoped SQLite](adrs/adr-002.md) — execution must validate the artifact/state split without reintroducing metadata-file assumptions
- [ADR-003: Expose the Daemon Through AGH-Aligned REST Transports Using Gin](adrs/adr-003.md) — execution must validate UDS/HTTP parity, error envelopes, and SSE behavior
- [ADR-004: Preserve TUI-First UX While Introducing Auto-Start and Explicit Workspace Operations](adrs/adr-004.md) — execution must validate `tasks run`, attach modes, and explicit workspace operations

## Deliverables
- Fresh `.compozy/tasks/daemon/qa/verification-report.md` produced by `/qa-execution`
- Fresh issue reports and supporting artifacts under `.compozy/tasks/daemon/qa/` for any regressions found **(REQUIRED)**
- Root-cause bug fixes plus matching regression tests for any issues discovered during execution **(REQUIRED)**
- Fresh evidence for daemon bootstrap, workspace, sync/archive, review, exec, attach/watch, transport parity, and public run-reader flows **(REQUIRED)**
- Passing `make verify` after the final QA fixes **(REQUIRED)**

## Tests
- Unit tests:
  - [ ] Contract discovery, execution notes, and verification artifacts capture the repository-defined daemon verification lane without command drift
  - [ ] Bug reports and verification-report entries use the shared QA templates and point to real daemon scenarios
  - [ ] Any new or updated regression tests target the specific daemon regression discovered during execution instead of broad brittle rewrites
  - [ ] Test-case annotations are updated when execution proves a flow is `Existing`, `Missing`, `Manual-only`, or `Blocked`
- Integration tests:
  - [ ] Daemon bootstrap/recovery, workspace registry, sync/archive, review, exec, attach/watch, and `pkg/compozy/runs` scenarios are exercised through real CLI/API entrypoints
  - [ ] UDS and HTTP parity or equivalent transport validation is exercised where the daemon surface exists on the execution branch
  - [ ] Any daemon regression found during execution is reproduced, fixed at the root, and protected by durable automated coverage
  - [ ] Browser validation is either executed against a real daemon web surface or reported explicitly as blocked/out of scope with the exact reason
  - [ ] `make verify` passes cleanly after the final QA fix set
- Test coverage target: >=80%
- All tests must pass

## Success Criteria
- The `/qa-execution` workflow has been run explicitly with artifacts stored under `.compozy/tasks/daemon/qa/`
- The daemon feature has fresh QA evidence for its critical operator surfaces
- Any QA failures were fixed at the source and documented with new evidence
- The normal repository verification gate passes from the current daemon branch state
- Future follow-up work can rely on `.compozy/tasks/daemon/qa/verification-report.md` instead of rediscovering the daemon QA contract
