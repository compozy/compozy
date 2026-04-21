---
status: pending
title: Daemon Web UI QA Plan and Regression Artifacts
type: docs
complexity: high
dependencies:
  - task_08
  - task_09
  - task_10
  - task_11
  - task_12
  - task_13
  - task_14
---

# Daemon Web UI QA Plan and Regression Artifacts

## Overview

Generate the reusable QA planning artifacts for the daemon web UI before live execution begins, with coverage centered on the real browser/operator flows delivered by the feature. This task should leave the repository with a concrete test plan, traceable execution cases, and regression-suite definitions rooted at `.compozy/tasks/daemon-web-ui/qa/` so the execution task can validate the feature without rediscovering scope.

<critical>
- ALWAYS READ `_techspec.md`, ADRs, and `task_08.md` through `task_14.md` before planning coverage
- ACTIVATE `/qa-report` with `qa-output-path=.compozy/tasks/daemon-web-ui` before writing or revising any QA artifact
- KEEP THE SAME `qa-output-path` FOR `/qa-execution` — all planning and execution artifacts must live under `.compozy/tasks/daemon-web-ui/qa/`
- FOCUS ON "WHAT" — define scope, risks, evidence layout, and automation priorities; do not execute browser or daemon flows in this task
- DO NOT TREAT THE BROWSER AS OPTIONAL — this feature has a real browser surface and a required E2E harness
- GREENFIELD: the regression matrix must cover the embedded daemon-served SPA, not only Vite or mocked route states
</critical>

<requirements>
1. MUST use the `/qa-report` skill with `qa-output-path=.compozy/tasks/daemon-web-ui`.
2. MUST generate a feature-level QA plan under `.compozy/tasks/daemon-web-ui/qa/test-plans/`.
3. MUST generate test cases covering dashboard, workflow inventory, task board/detail, run list/detail/live watch, reviews, spec, memory, workspace selection, sync/archive, run start/cancel, and review-fix flows.
4. MUST classify each critical browser/operator flow as `E2E`, `Integration`, `Manual-only`, or `Blocked` using the real harness and repo constraints.
5. MUST generate at least one regression-suite document identifying the smoke, targeted, and full execution priorities for the daemon web UI.
6. SHOULD call out any remaining automation gaps explicitly instead of downgrading critical browser flows to manual without a concrete blocker.
</requirements>

## Subtasks
- [ ] 15.1 Activate `/qa-report` with `qa-output-path=.compozy/tasks/daemon-web-ui`.
- [ ] 15.2 Write the feature test plan with scope, environments, risks, and entry/exit criteria for the daemon web UI.
- [ ] 15.3 Generate traceable test cases for the critical browser/operator flows and route states.
- [ ] 15.4 Build regression-suite definitions and identify the P0/P1 flows that execution must validate first.
- [ ] 15.5 Validate artifact completeness, traceability, and handoff readiness for the execution task.

## Implementation Details

See the TechSpec sections "Testing Approach", "Development Sequencing", "Known Risks", and "Technical Dependencies". This task is the formal QA handoff for the daemon web UI and must assume a real browser surface, embedded serving, Storybook/MSW, and a repo-supported Playwright harness already exist.

### Relevant Files
- `.agents/skills/qa-report/SKILL.md` — required planning workflow and artifact layout.
- `.compozy/tasks/daemon-web-ui/_techspec.md` — source of truth for browser scope, verification layers, and critical flows.
- `.compozy/tasks/daemon-web-ui/task_09.md` — shell, workspace selection, and dashboard coverage anchor.
- `.compozy/tasks/daemon-web-ui/task_10.md` — run list/detail/live watch/start/cancel coverage anchor.
- `.compozy/tasks/daemon-web-ui/task_11.md` — workflow task board/detail coverage anchor.
- `.compozy/tasks/daemon-web-ui/task_12.md` — reviews, spec, memory, and review-fix coverage anchor.

### Dependent Files
- `.compozy/tasks/daemon-web-ui/qa/test-plans/daemon-web-ui-test-plan.md` — feature-level QA plan created by this task.
- `.compozy/tasks/daemon-web-ui/qa/test-plans/*-regression.md` — regression suite documents consumed by the execution task.
- `.compozy/tasks/daemon-web-ui/qa/test-cases/TC-*.md` — execution-ready browser/operator test cases.
- `.compozy/tasks/daemon-web-ui/task_16.md` — execution task that must consume this artifact set unchanged.

### Related ADRs
- [ADR-002: Serve the Embedded SPA from the Daemon's Existing HTTP Listener](adrs/adr-002.md) — QA planning must cover the daemon-served topology.
- [ADR-005: Require Full Frontend Verification with Vitest, Playwright, Storybook, and MSW](adrs/adr-005.md) — QA planning must account for the full verification and browser coverage bar.

## Deliverables
- `.compozy/tasks/daemon-web-ui/qa/test-plans/daemon-web-ui-test-plan.md`
- One or more `.compozy/tasks/daemon-web-ui/qa/test-plans/*-regression.md` documents reusable by `/qa-execution`
- Traceable browser/operator test cases under `.compozy/tasks/daemon-web-ui/qa/test-cases/` **(REQUIRED)**
- Explicit P0/P1 regression coverage for dashboard, workflows, runs, reviews, spec, memory, workspace selection, and operational actions **(REQUIRED)**
- Stable artifact layout under `.compozy/tasks/daemon-web-ui/qa/` that the execution task can consume without path changes **(REQUIRED)**

## Tests
- Unit tests:
  - [ ] The test plan includes objectives, scope, environment matrix, entry/exit criteria, and risk assessment for the daemon web UI.
  - [ ] Test cases exist for dashboard, workflows, task detail, runs/live watch, reviews, spec, memory, workspace selection, sync/archive, and run/review operations.
  - [ ] Each test case includes preconditions, steps, expected results, priority, and automation annotations.
  - [ ] Regression-suite documents identify smoke, targeted, and full coverage with clear P0/P1 priorities.
- Integration tests:
  - [ ] All generated artifacts land under `.compozy/tasks/daemon-web-ui/qa/` and can be consumed directly by `/qa-execution`.
  - [ ] Test cases trace back to relevant TechSpec sections, ADRs, or task deliverables clearly.
  - [ ] Regression suites and test plans reference the same case IDs, priorities, and artifact paths without naming drift.
- Test coverage target: >=80%
- All tests must pass

## Success Criteria
- All tests passing
- Test coverage >=80%
- The daemon web UI has a concrete QA plan and regression matrix rooted at the feature-local `qa/` directory
- Critical browser/operator flows have explicit automation classifications and priorities
- The execution task can start without redefining the QA scope or artifact layout
