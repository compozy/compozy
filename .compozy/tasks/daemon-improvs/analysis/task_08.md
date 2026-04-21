---
status: pending
title: Daemon Improvements QA plan and regression artifacts
type: docs
complexity: high
dependencies:
  - task_03
  - task_04
  - task_05
  - task_06
  - task_07
---

# Daemon Improvements QA plan and regression artifacts

## Overview

Generate the reusable QA planning artifacts for the daemon hardening and contract migration before live execution begins. This task should leave the repo with a concrete test plan, traceable execution cases, and regression-suite definitions covering transport parity, runtime supervision, snapshot integrity, and client or run-reader compatibility, all stored under the same feature-local QA root that the execution task will reuse.

<critical>
- ALWAYS READ `_techspec.md`, ADRs, and `task_03.md` through `task_07.md` before planning coverage
- ACTIVATE `/qa-report` with `qa-output-path=.compozy/tasks/daemon-improvs/analysis` before writing or revising any QA artifact
- KEEP THE SAME `qa-output-path` FOR `/qa-execution` - all planning and execution artifacts must live under `.compozy/tasks/daemon-improvs/analysis/qa/`
- FOCUS ON "WHAT" - define coverage, risks, automation targets, and evidence layout; do not execute daemon flows or pre-emptively fix bugs in this task
- DO NOT INVENT A BROWSER SURFACE - if no daemon web UI exists on the execution branch, mark browser validation as blocked or out of scope explicitly
- E2E FOLLOW-UP IS REQUIRED - critical public daemon flows must be classified explicitly as `E2E`, `Integration`, `Manual-only`, or `Blocked`, with reasons
</critical>

<requirements>
1. MUST use the `/qa-report` skill with `qa-output-path=.compozy/tasks/daemon-improvs/analysis`.
2. MUST generate a feature-level test plan under `.compozy/tasks/daemon-improvs/analysis/qa/test-plans/`.
3. MUST generate test cases covering canonical transport contract behavior, HTTP or UDS parity, daemon client timeout classes, run snapshot and replay behavior, runtime shutdown and log policy, ACP liveness and fault scenarios, and observability surfaces including health, metrics, and transcript assembly.
4. MUST classify each critical daemon-hardening flow as `E2E`, `Integration`, `Manual-only`, or `Blocked` according to the repository's real harnesses, and it MUST call out which P0 or P1 public flows still need E2E follow-up.
5. MUST produce at least one regression-suite document defining smoke, targeted, and full execution priorities for the daemon-improvement flows, including `make verify`, transport parity checks, and E2E or integration follow-up for supported critical surfaces.
6. SHOULD mark browser validation as blocked or out of scope unless a real daemon web surface exists on the execution branch.
</requirements>

## Subtasks

- [ ] 8.1 Activate `/qa-report` with `qa-output-path=.compozy/tasks/daemon-improvs/analysis`.
- [ ] 8.2 Write the feature test plan with scope, risks, environments, automation strategy, and entry or exit criteria.
- [ ] 8.3 Generate traceable test cases for transport parity, client and run-reader compatibility, lifecycle hardening, ACP fault handling, and observability flows.
- [ ] 8.4 Build regression-suite definitions and identify the P0 or P1 flows that `/qa-execution` must run first, including explicit E2E follow-up where the harness supports it.
- [ ] 8.5 Validate artifact completeness, traceability, and handoff readiness for `task_09`.

## Implementation Details

See TechSpec sections "Testing Approach", "Test Lanes", "Monitoring and Observability", "Known Risks", and "Development Sequencing". This task is the formal handoff from the daemon-improvement implementation tasks to QA execution: it should capture what must be proven for the contract migration and runtime hardening, which flows require E2E follow-up, and where all evidence must live.

### Relevant Files

- `.agents/skills/qa-report/SKILL.md` - required planning workflow, shared artifact structure, and automation annotation rules.
- `.agents/skills/qa-report/references/test_case_templates.md` - source template for the feature test plan and individual test cases.
- `.agents/skills/qa-report/references/regression_testing.md` - source template for smoke, targeted, and full regression suites.
- `.compozy/tasks/daemon-improvs/analysis/_techspec.md` - source of truth for scope, sequencing, contracts, runtime hardening, and observability requirements.
- `.compozy/tasks/daemon-improvs/analysis/task_03.md` - shared transport migration behavior that QA must cover for parity and contract correctness.
- `.compozy/tasks/daemon-improvs/analysis/task_04.md` - daemon client and `pkg/compozy/runs` compatibility behavior that QA must cover.
- `.compozy/tasks/daemon-improvs/analysis/task_05.md` - lifecycle, logging, and checkpoint discipline that QA must validate.
- `.compozy/tasks/daemon-improvs/analysis/task_06.md` - ACP liveness, subprocess supervision, and reconcile hardening that QA must validate.
- `.compozy/tasks/daemon-improvs/analysis/task_07.md` - health, metrics, snapshot integrity, and transcript assembly behavior that QA must validate.

### Dependent Files

- `.compozy/tasks/daemon-improvs/analysis/qa/test-plans/daemon-improvs-analysis-test-plan.md` - feature-level QA plan created by this task.
- `.compozy/tasks/daemon-improvs/analysis/qa/test-plans/*-regression.md` - regression-suite document(s) consumed by the execution task.
- `.compozy/tasks/daemon-improvs/analysis/qa/test-cases/TC-*.md` - execution-ready test cases with priorities and automation annotations.
- `.compozy/tasks/daemon-improvs/analysis/qa/issues/BUG-*.md` - only created if planning uncovers a documented discrepancy worth filing immediately.
- `.compozy/tasks/daemon-improvs/analysis/task_09.md` - execution task that must consume this artifact set unchanged.

### Related ADRs

- [ADR-001: Canonical Daemon Transport Contract](adrs/adr-001.md) - QA planning must cover canonical contracts, parity, and public reader compatibility.
- [ADR-002: Incremental Runtime Supervision Hardening Inside the Existing Daemon Boundary](adrs/adr-002.md) - QA planning must cover shutdown, recovery, subprocess handling, and checkpoint behavior.
- [ADR-003: Validation-First Daemon Hardening](adrs/adr-003.md) - QA planning must call out real-daemon parity, ACP fault injection, and E2E follow-up for supported critical flows.
- [ADR-004: Observability as a First-Class Daemon Contract](adrs/adr-004.md) - QA planning must cover health, metrics, snapshots, transcript assembly, and integrity semantics.

## Deliverables

- `.compozy/tasks/daemon-improvs/analysis/qa/test-plans/daemon-improvs-analysis-test-plan.md`
- One or more `.compozy/tasks/daemon-improvs/analysis/qa/test-plans/*-regression.md` documents reusable by `/qa-execution`
- Traceable test cases under `.compozy/tasks/daemon-improvs/analysis/qa/test-cases/` **(REQUIRED)**
- Explicit P0 or P1 regression coverage for transport parity, client and run-reader compatibility, runtime shutdown and recovery, ACP fault handling, and observability flows **(REQUIRED)**
- A stable artifact layout under `.compozy/tasks/daemon-improvs/analysis/qa/` that the execution task can consume without path changes **(REQUIRED)**

## Tests

- Unit tests:
  - [ ] `daemon-improvs-analysis-test-plan.md` includes objectives, scope, environment matrix, automation strategy, entry or exit criteria, risk assessment, and explicit artifact ownership.
  - [ ] Test cases exist for transport parity, timeout-class behavior, run snapshot and replay behavior, shutdown and checkpoint discipline, ACP stall and fault scenarios, and observability surfaces.
  - [ ] Each test case includes preconditions, steps, expected results, priority, and automation annotations suitable for `/qa-execution`.
  - [ ] Regression-suite documents identify smoke, targeted, and full coverage, including execution order and E2E follow-up expectations for P0 or P1 public flows.
  - [ ] Browser validation is explicitly marked as blocked or out of scope when no daemon web surface is available.
- Integration tests:
  - [ ] All generated artifacts land under `.compozy/tasks/daemon-improvs/analysis/qa/` and can be consumed directly by `/qa-execution`.
  - [ ] Test cases trace back to the relevant daemon-improvement tasks, TechSpec sections, or ADR decisions clearly.
  - [ ] Regression-suite and test-plan documents reference the same case IDs, priorities, artifact paths, and E2E classifications without naming drift.
  - [ ] Any bug report created during planning references the originating test case or documented discrepancy clearly.
- Test coverage target: >=80%
- All tests must pass

## Success Criteria

- The `/qa-report` workflow has been executed explicitly and its artifacts are stored under `.compozy/tasks/daemon-improvs/analysis/qa/`
- Every daemon-improvement critical flow has at least one traceable QA artifact
- `task_09` can start execution without redefining scope, output paths, or automation priorities
- The daemon-improvement workflow has a concrete regression plan instead of ad hoc QA notes
