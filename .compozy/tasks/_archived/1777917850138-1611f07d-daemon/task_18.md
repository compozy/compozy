---
status: completed
title: Daemon QA plan and regression artifacts
type: docs
complexity: high
dependencies:
  - task_12
  - task_13
  - task_14
  - task_15
  - task_16
  - task_17
---

# Task 18: Daemon QA plan and regression artifacts

## Overview

Generate the reusable QA planning artifacts for the daemon migration before live execution begins, with coverage centered on operator-visible CLI, API, TUI, sync/archive, and public run-reader behavior. This task should leave the repo with a concrete daemon test plan, traceable execution cases, and regression-suite definitions that the follow-up execution task can consume without redefining scope or output paths.

<critical>
- ALWAYS READ `_techspec.md`, ADRs, and `task_12.md` through `task_17.md` before planning coverage
- ACTIVATE `/qa-report` with `qa-output-path=.compozy/tasks/daemon` before writing or revising any QA artifact
- KEEP THE SAME `qa-output-path` FOR `/qa-execution` — all planning and execution artifacts must live under `.compozy/tasks/daemon/qa/`
- FOCUS ON "WHAT" — define coverage, risks, and evidence layout; do not execute daemon flows or pre-emptively fix bugs in this task
- DO NOT INVENT A BROWSER SURFACE — if no daemon web UI exists on the execution branch, mark browser validation as blocked or out of scope explicitly
- GREENFIELD: the regression matrix must cover the final daemon-native control plane, not legacy `compozy start`, `_tasks.md`, or `_meta.md` assumptions
</critical>

<requirements>
1. MUST use the `/qa-report` skill with `qa-output-path=.compozy/tasks/daemon`.
2. MUST generate a feature-level test plan under `.compozy/tasks/daemon/qa/test-plans/`.
3. MUST generate test cases covering daemon bootstrap and recovery, workspace registry flows, task runs, review runs, exec runs, sync and archive behavior, attach/watch behavior, transport parity, and `pkg/compozy/runs` compatibility.
4. MUST classify each critical daemon flow as `E2E`, `Integration`, `Manual-only`, or `Blocked` according to the repository’s real harnesses; do not invent browser or TUI automation commands when the harness does not exist.
5. MUST produce at least one regression-suite document defining smoke, targeted, and full execution priorities for daemon-critical flows, including `make verify` and any explicit CLI/API parity checks.
6. SHOULD call out browser validation as blocked or out of scope unless a real daemon web surface exists on the execution branch.
</requirements>

## Subtasks
- [x] 18.1 Activate `/qa-report` with `qa-output-path=.compozy/tasks/daemon`.
- [x] 18.2 Write the daemon feature test plan with scope, risks, environments, and entry/exit criteria.
- [x] 18.3 Generate traceable test cases for daemon bootstrap, workspace, sync/archive, run lifecycle, attach/watch, review, exec, performance, and public run-reader flows.
- [x] 18.4 Build regression-suite definitions and identify the P0/P1 flows that `/qa-execution` must run first.
- [x] 18.5 Validate artifact completeness, traceability, and handoff readiness for `task_19`.

## Implementation Details

See TechSpec sections "Testing Approach", "Development Sequencing", "Known Risks", "CLI/TUI clients", and "Transport Contract". This task is the formal handoff from daemon implementation to QA execution: it should capture what must be proven for the daemon migration, which flows are automation candidates, and where all evidence must live.

### Relevant Files
- `.agents/skills/qa-report/SKILL.md` — required planning workflow, artifact structure, and naming rules
- `.agents/skills/qa-report/references/test_case_templates.md` — source template for feature test plans and case structure
- `.agents/skills/qa-report/references/regression_testing.md` — source template for smoke, targeted, and full regression suites
- `.compozy/tasks/daemon/_techspec.md` — source of truth for daemon testing scope, sequence, and risks
- `.compozy/tasks/daemon/task_12.md` — defines TUI attach/watch behavior that QA must classify correctly
- `.compozy/tasks/daemon/task_13.md` — defines `pkg/compozy/runs` migration behavior that QA must cover
- `.compozy/tasks/daemon/task_14.md` — defines daemon, workspace, sync, and archive command flows that QA must cover
- `.compozy/tasks/daemon/task_15.md` — defines review and exec flows that QA must cover
- `.compozy/tasks/daemon/task_16.md` — defines daemon performance optimization surfaces that QA must validate for regressions and expected wins
- `.compozy/tasks/daemon/task_17.md` — defines migration-closeout expectations that the QA plan must turn into executable evidence

### Dependent Files
- `.compozy/tasks/daemon/qa/test-plans/daemon-test-plan.md` — feature-level QA plan created by this task
- `.compozy/tasks/daemon/qa/test-plans/*-regression.md` — regression-suite document(s) consumed by the execution task
- `.compozy/tasks/daemon/qa/test-cases/TC-*.md` — execution-ready test cases with priorities and automation annotations
- `.compozy/tasks/daemon/qa/issues/BUG-*.md` — only created if planning uncovers a concrete documented discrepancy
- `.compozy/tasks/daemon/task_19.md` — execution task that must consume this artifact set unchanged

### Related ADRs
- [ADR-001: Adopt a Global Home-Scoped Singleton Daemon](adrs/adr-001.md) — QA planning must cover singleton boot, recovery, and workspace coordination
- [ADR-002: Keep Human Artifacts in the Workspace and Move Operational State to Home-Scoped SQLite](adrs/adr-002.md) — QA planning must distinguish workspace artifacts from daemon-owned operational state
- [ADR-003: Expose the Daemon Through AGH-Aligned REST Transports Using Gin](adrs/adr-003.md) — QA planning must include UDS/HTTP parity and SSE behavior
- [ADR-004: Preserve TUI-First UX While Introducing Auto-Start and Explicit Workspace Operations](adrs/adr-004.md) — QA planning must cover attach-mode defaults, `tasks run`, and explicit workspace operations

## Deliverables
- `.compozy/tasks/daemon/qa/test-plans/daemon-test-plan.md`
- One or more `.compozy/tasks/daemon/qa/test-plans/*-regression.md` documents reusable by `/qa-execution`
- Traceable test cases under `.compozy/tasks/daemon/qa/test-cases/` **(REQUIRED)**
- Explicit P0/P1 regression coverage for daemon bootstrap, CLI/API parity, workspace flows, sync/archive, attach/watch, review, exec, and public run readers **(REQUIRED)**
- A stable artifact layout under `.compozy/tasks/daemon/qa/` that the execution task can consume without path changes **(REQUIRED)**

## Tests
- Unit tests:
  - [x] `daemon-test-plan.md` includes objectives, scope, environment matrix, entry/exit criteria, risk assessment, and explicit artifact ownership
  - [x] Test cases exist for daemon bootstrap/recovery, workspace registration, task runs, review runs, exec, sync/archive, attach/watch, transport parity, and `pkg/compozy/runs`
  - [x] Each test case includes preconditions, steps, expected results, priority, and automation annotations suitable for `/qa-execution`
  - [x] Regression-suite documents identify smoke, targeted, and full coverage, including execution order and blocker expectations for P0/P1 daemon flows
  - [x] Browser validation is explicitly marked as blocked or out of scope when no daemon web surface is available
- Integration tests:
  - [x] All generated artifacts land under `.compozy/tasks/daemon/qa/` and can be consumed directly by `/qa-execution`
  - [x] Test cases trace back to the relevant daemon tasks, TechSpec sections, or ADR decisions clearly
  - [x] Regression-suite and test-plan documents reference the same case IDs, priorities, and artifact paths without naming drift
  - [x] Any bug report created during planning references the originating test case or documented discrepancy clearly
- Test coverage target: >=80%
- All tests must pass

## Success Criteria
- The `/qa-report` workflow has been executed explicitly and its artifacts are stored under `.compozy/tasks/daemon/qa/`
- Every daemon-critical operator flow has at least one traceable QA artifact
- `task_19` can start execution without redefining scope, output paths, or risk priorities
- The daemon feature has a concrete regression plan instead of ad hoc QA notes
