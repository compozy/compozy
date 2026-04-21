---
status: pending
title: Daemon Web UI QA Execution and Browser/Operator Validation
type: test
complexity: critical
dependencies:
  - task_15
---

# Daemon Web UI QA Execution and Browser/Operator Validation

## Overview

Execute the full QA pass for the daemon web UI using the artifacts from `task_15`, and leave fresh evidence for the browser, API, and operator-facing flows. This task is responsible for turning the plan into real validation, fixing root-cause regressions when they appear, and publishing a verification report backed by current repository evidence.

<critical>
- ALWAYS READ `_techspec.md`, ADRs, and the QA artifacts from `task_15` before running validation
- ACTIVATE `/qa-execution` with `qa-output-path=.compozy/tasks/daemon-web-ui` before any live verification or evidence capture
- IF QA FINDS A BUG, ACTIVATE `/systematic-debugging` AND `/no-workarounds` BEFORE CHANGING CODE OR TESTS
- FOLLOW THE REPOSITORY QA CONTRACT — use the discovered verify/build/test/start commands, real daemon/browser entrypoints, and the existing Playwright harness
- DO NOT DOWNGRADE THE BROWSER LANE — execute flows against daemon-served embedded assets, not just Vite dev mode
- DO NOT WEAKEN TESTS TO GET GREEN — fix production code or real configuration at the source, then rerun the impacted scenarios and full gates
</critical>

<requirements>
1. MUST use the `/qa-execution` skill with `qa-output-path=.compozy/tasks/daemon-web-ui`.
2. MUST consume `.compozy/tasks/daemon-web-ui/qa/test-plans/` and `.compozy/tasks/daemon-web-ui/qa/test-cases/` from `task_15` as the execution matrix seed.
3. MUST validate dashboard, workflow inventory, task board/detail, run list/detail/live watch, reviews, spec, memory, workspace selection, sync/archive, run start/cancel, and review-fix flows using real repository entrypoints.
4. MUST execute browser flows against daemon-served embedded assets and capture screenshots/logs/issues under `.compozy/tasks/daemon-web-ui/qa/`.
5. MUST write fresh QA evidence to `.compozy/tasks/daemon-web-ui/qa/verification-report.md`.
6. MUST add or update regression coverage for any discovered public browser or operator regressions using the narrowest durable test surface, including E2E updates when the harness exists.
7. MUST rerun the repository verification gates after the last fix, including the full `make verify` lane from the current branch state.
</requirements>

## Subtasks
- [ ] 16.1 Activate `/qa-execution` with `qa-output-path=.compozy/tasks/daemon-web-ui` and derive the execution matrix from the planning artifacts.
- [ ] 16.2 Discover the repository QA contract, establish the baseline, and document any blockers before scenario testing.
- [ ] 16.3 Execute browser, API, and operator flows against the embedded daemon-served UI with durable evidence capture.
- [ ] 16.4 Fix root-cause regressions with matching durable coverage and rerun the impacted flows.
- [ ] 16.5 Rerun `make verify`, finalize the verification report, and publish any issue artifacts or blockers.

## Implementation Details

See the TechSpec sections "Testing Approach", "Development Sequencing", "Technical Dependencies", and "Known Risks". The key constraint is that daemon web UI QA must rely on the real repo gate and the real embedded browser runtime, not on ad hoc local checks that cannot be repeated later.

### Relevant Files
- `.agents/skills/qa-execution/SKILL.md` — required execution workflow, repository discovery, and verification reporting.
- `.compozy/tasks/daemon-web-ui/qa/test-plans/` — planning artifacts that seed the execution matrix.
- `.compozy/tasks/daemon-web-ui/qa/test-cases/` — test cases and automation annotations that execution must honor.
- `Makefile` — repository-defined verification gate that must pass from the current branch state.
- `web/playwright.config.ts` — browser E2E harness that execution must use or validate.
- `web/e2e/` — existing browser/operator regression coverage that execution may need to extend after fixes.

### Dependent Files
- `.compozy/tasks/daemon-web-ui/qa/verification-report.md` — final QA evidence written by `/qa-execution`.
- `.compozy/tasks/daemon-web-ui/qa/screenshots/` — screenshot evidence for browser flows and regressions.
- `.compozy/tasks/daemon-web-ui/qa/issues/BUG-*.md` — structured bug reports for failures discovered during execution.
- `web/e2e/` — may gain new or updated regression coverage for public browser regressions.

### Related ADRs
- [ADR-002: Serve the Embedded SPA from the Daemon's Existing HTTP Listener](adrs/adr-002.md) — execution must validate the embedded daemon-served browser topology.
- [ADR-005: Require Full Frontend Verification with Vitest, Playwright, Storybook, and MSW](adrs/adr-005.md) — execution must validate the feature against the full browser/operator verification bar.

## Deliverables
- Fresh `.compozy/tasks/daemon-web-ui/qa/verification-report.md` produced by `/qa-execution`
- Fresh issue reports and supporting artifacts under `.compozy/tasks/daemon-web-ui/qa/` for any regressions found **(REQUIRED)**
- Root-cause bug fixes plus matching regression coverage for any issues discovered during execution **(REQUIRED)**
- Fresh evidence for dashboard, workflows, tasks, runs/live watch, reviews, spec, memory, workspace selection, and operational browser flows **(REQUIRED)**
- Passing `make verify` after the final QA fix set **(REQUIRED)**

## Tests
- Unit tests:
  - [ ] Contract discovery, execution notes, and verification artifacts capture the repository-defined web-ui verification lane without command drift.
  - [ ] Bug reports and verification-report entries use the shared QA templates and point to real browser/operator scenarios.
  - [ ] Any new or updated regression tests target the specific regressions discovered during execution.
- Integration tests:
  - [ ] Browser flows run against daemon-served embedded assets, not just a Vite dev server.
  - [ ] Dashboard, workflows, tasks, runs/live watch, reviews, spec, memory, and operational actions are exercised through real browser/API entrypoints.
  - [ ] Any public regression found during execution is reproduced, fixed at the root, and protected by durable automated coverage.
  - [ ] `make verify` passes cleanly after the final QA fix set.
- Test coverage target: >=80%
- All tests must pass

## Success Criteria
- All tests passing
- Test coverage >=80%
- The daemon web UI has fresh QA evidence for its critical browser and operator surfaces
- Any QA failures were fixed at the source and documented with new evidence
- The repository verification gate passes from the current daemon-web-ui branch state
