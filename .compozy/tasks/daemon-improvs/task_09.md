---
status: completed
title: Daemon Improvements QA execution and operator-flow validation
type: test
complexity: critical
dependencies:
  - task_08
---

# Daemon Improvements QA execution and operator-flow validation

## Overview

Execute the full QA pass for the daemon hardening workflow using the artifacts from `task_08`, and leave fresh evidence for the CLI, API, transport, runtime, and run-reader surfaces affected by the TechSpec. This task is responsible for turning the QA plan into real validation, fixing root-cause regressions when they appear, and publishing a verification report backed by current repository evidence and E2E or integration coverage for supported critical flows.

<critical>
- ALWAYS READ `_techspec.md`, ADRs, and the QA artifacts from `task_08` before running any validation
- ACTIVATE `/qa-execution` with `qa-output-path=.compozy/tasks/daemon-improvs/analysis` before any live verification or evidence capture
- IF QA FINDS A BUG, ACTIVATE `/systematic-debugging` AND `/no-workarounds` BEFORE CHANGING CODE OR TESTS
- FOLLOW THE REPOSITORY QA CONTRACT - use the discovered verify/build/test commands, real CLI and API entrypoints, and existing integration harnesses instead of one-off scripts
- DO NOT INVENT A BROWSER LANE - if no daemon web surface exists on the execution branch, report browser validation as blocked or out of scope explicitly
- DO NOT WEAKEN TESTS TO GET GREEN - fix production code or configuration at the source, then rerun the affected scenarios and full gates
- E2E FOLLOW-UP IS REQUIRED - for supported P0 or P1 public flows, add or update the smallest repo-supported E2E or public-flow automated coverage rather than stopping at manual proof
</critical>

<requirements>
1. MUST use the `/qa-execution` skill with `qa-output-path=.compozy/tasks/daemon-improvs/analysis`.
2. MUST consume `.compozy/tasks/daemon-improvs/analysis/qa/test-plans/` and `.compozy/tasks/daemon-improvs/analysis/qa/test-cases/` from `task_08` as the execution matrix seed.
3. MUST validate canonical transport contract behavior, HTTP or UDS parity, daemon client timeout classes, run snapshot or replay behavior, runtime shutdown and logging behavior, ACP liveness and fault handling, and observability surfaces using real repository entrypoints.
4. MUST validate at least one daemon-backed operator flow against a temporary external workspace, using a minimal realistic workspace fixture to exercise task or review execution plus run inspection under the hardened daemon surface.
5. MUST write fresh QA evidence to `.compozy/tasks/daemon-improvs/analysis/qa/verification-report.md` and capture bugs or screenshots or logs under the same artifact root when relevant.
6. MUST add or update regression coverage for any supported critical public regression discovered during execution, including E2E or the smallest repository-supported public-flow automated coverage when the harness exists.
7. MUST rerun the repository verification gates after the last fix, including `make verify`, and include the executed commands in the final verification report.
8. SHOULD record browser flows as `Blocked` or `Manual-only` only when the repository truly lacks a stable automation harness for that surface.
</requirements>

## Subtasks

- [x] 9.1 Activate `/qa-execution` with `qa-output-path=.compozy/tasks/daemon-improvs/analysis` and derive the execution matrix from `task_08` artifacts.
- [x] 9.2 Discover the repository QA contract, establish the daemon baseline, and document any existing blockers before scenario testing.
- [x] 9.3 Execute CLI, API, transport-parity, runtime hardening, snapshot or replay, ACP fault, and external-workspace scenarios with durable evidence capture.
- [x] 9.4 Fix root-cause regressions with matching regression or E2E coverage and rerun the impacted scenarios.
- [x] 9.5 Rerun `make verify`, finalize `.compozy/tasks/daemon-improvs/analysis/qa/verification-report.md`, and publish any bug artifacts or blockers.

## Implementation Details

See TechSpec sections "Testing Approach", "Test Lanes", "Known Risks", "Monitoring and Observability", and "Development Sequencing". The key constraint is that daemon-improvement QA must rely on the real repo verification contract and real operator surfaces, not exploratory one-off checks that cannot be repeated later.

### Relevant Files

- `.agents/skills/qa-execution/SKILL.md` - required workflow for repository-contract discovery, evidence capture, and verification reporting.
- `.agents/skills/qa-execution/scripts/discover-project-contract.py` - canonical repo-contract discovery entrypoint required by `/qa-execution`.
- `.agents/skills/qa-execution/references/checklist.md` - required execution-scope checklist for QA completeness.
- `.agents/skills/qa-execution/references/e2e-coverage.md` - required source for deciding whether a harness exists for critical daemon flows.
- `.agents/skills/qa-execution/assets/verification-report-template.md` - required verification report structure.
- `.agents/skills/qa-execution/assets/issue-template.md` - required bug report template shared with `qa-report`.
- `.compozy/tasks/daemon-improvs/analysis/qa/test-plans/` - planning artifacts that seed the execution matrix.
- `.compozy/tasks/daemon-improvs/analysis/qa/test-cases/` - test cases and automation annotations that the execution pass must honor.
- `Makefile` - repository-defined verification gate that must pass before completion.
- `internal/api/httpapi/transport_integration_test.go` - parity seam for HTTP transport validation and automation follow-up.
- `internal/core/run/executor/execution_acp_integration_test.go` - ACP integration seam for fault-handling validation and regression coverage.
- `pkg/compozy/runs/integration_test.go` - public run-reader integration coverage that must prove daemon-backed behavior end to end.

### Dependent Files

- `.compozy/tasks/daemon-improvs/analysis/qa/verification-report.md` - final QA evidence written by `/qa-execution`.
- `.compozy/tasks/daemon-improvs/analysis/qa/screenshots/` - screenshot evidence when a visual or terminal artifact is captured.
- `.compozy/tasks/daemon-improvs/analysis/qa/issues/BUG-*.md` - structured bug reports for failures discovered during execution.
- `.compozy/tasks/daemon-improvs/analysis/qa/test-cases/*.md` - may be updated if execution reveals missing or blocked automation annotations.
- `internal/api/*_test.go` - may gain regression or parity coverage if execution finds transport issues.
- `internal/core/run/executor/*_test.go` - may gain regression coverage for ACP liveness or runtime supervision issues.
- `pkg/compozy/runs/*_test.go` - may gain regression coverage for public run-reader regressions.

### Related ADRs

- [ADR-001: Canonical Daemon Transport Contract](adrs/adr-001.md) - execution must validate contract parity, error envelopes, and public reader compatibility.
- [ADR-002: Incremental Runtime Supervision Hardening Inside the Existing Daemon Boundary](adrs/adr-002.md) - execution must validate shutdown, recovery, subprocess handling, and checkpoint behavior.
- [ADR-003: Validation-First Daemon Hardening](adrs/adr-003.md) - execution must validate the real-daemon harness, parity, ACP fault scenarios, and E2E follow-up for supported critical flows.
- [ADR-004: Observability as a First-Class Daemon Contract](adrs/adr-004.md) - execution must validate health, metrics, snapshots, transcript assembly, and integrity behavior.

## Deliverables

- Fresh `.compozy/tasks/daemon-improvs/analysis/qa/verification-report.md` produced by `/qa-execution`
- Fresh issue reports and supporting artifacts under `.compozy/tasks/daemon-improvs/analysis/qa/` for any regressions found **(REQUIRED)**
- Root-cause bug fixes plus matching regression or E2E tests for any issues discovered during execution **(REQUIRED)**
- Fresh evidence for transport parity, client and run-reader compatibility, runtime shutdown or recovery, ACP fault handling, observability flows, and one temporary external-workspace operator flow **(REQUIRED)**
- Passing `make verify` after the final QA fixes **(REQUIRED)**

## Tests

- Unit tests:
  - [x] Contract discovery, execution notes, and verification artifacts capture the repository-defined daemon verification lane without command drift.
  - [x] Bug reports and verification-report entries use the shared QA templates and point to real daemon-improvement scenarios.
  - [x] Any new or updated regression or E2E tests target the specific public regression discovered during execution instead of broad brittle rewrites.
  - [x] Test-case annotations are updated when execution proves a flow is `Existing`, `Missing`, `Manual-only`, or `Blocked`.
- Integration tests:
  - [x] Transport parity, client timeout behavior, snapshot or replay, ACP supervision, and observability scenarios are exercised through real CLI or API entrypoints and the shared daemon harness.
  - [x] At least one external-workspace operator flow is exercised end to end against the hardened daemon surface.
  - [x] Any daemon-improvement regression found during execution is reproduced, fixed at the root, and protected by durable automated coverage, including E2E where the harness supports it.
  - [x] Browser validation is either executed against a real daemon web surface or reported explicitly as blocked or out of scope with the exact reason.
  - [x] `make verify` passes cleanly after the final QA fix set.
- Test coverage target: >=80%
- All tests must pass

## Success Criteria

- The `/qa-execution` workflow has been run explicitly with artifacts stored under `.compozy/tasks/daemon-improvs/analysis/qa/`
- The daemon-improvement workflow has fresh QA evidence for its critical operator surfaces
- Any QA failures were fixed at the source and documented with new evidence
- The normal repository verification gate passes from the current daemon-improvement branch state
- Future follow-up work can rely on `.compozy/tasks/daemon-improvs/analysis/qa/verification-report.md` instead of rediscovering the QA contract
