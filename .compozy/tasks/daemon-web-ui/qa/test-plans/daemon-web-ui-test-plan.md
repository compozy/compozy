# Daemon Web UI QA Test Plan

## Executive Summary

This plan defines the reusable QA handoff for the daemon web UI before live execution begins in `task_16`. It fixes the artifact root at `.compozy/tasks/daemon-web-ui/qa/`, assigns stable test-case IDs, and maps each critical browser/operator flow to the real harness that exists in this worktree: the daemon-served Playwright smoke lane from `task_14`, the route/system integration suites from `task_09` through `task_12`, and the Storybook/MSW route-state harness from `task_13`.

The planning posture is evidence-driven. The browser is **not optional** for this feature, so the primary execution lane is the daemon-served embedded SPA at `/`, not Vite dev mode. Workspace bootstrap, review-fix dispatch, memory-file selection, run cancel, and stream reconnect/overflow are currently validated through integration tests. `task_16` closed the final browser gap by wiring workflow run start into `/workflows`, validating the daemon-served POST path, and promoting `TC-FUNC-008` from blocked coverage to executable E2E.

## Objectives

- Prove the embedded daemon-served SPA is ready for real browser QA without redefining scope in `task_16`.
- Give `task_16` stable, execution-ready case IDs for dashboard, workflows, tasks, runs, reviews, spec, memory, workspace selection, and operational actions.
- Classify every critical browser/operator flow as `E2E`, `Integration`, `Manual-only`, or `Blocked` from repository evidence rather than assumption.
- Preserve a fixed artifact layout for plans, cases, screenshots, issues, logs, and the final verification report.
- Make remaining gaps explicit so execution can either validate them through the real harness or file them as bugs with no ambiguity.

## Scope

### In Scope

- Daemon-served browser validation against the embedded SPA and existing Playwright harness.
- Workspace bootstrap, workspace switching, stale-workspace recovery, and shell navigation.
- Dashboard and workflow inventory rendering.
- Workflow sync, archive, board/detail drill-down, and related-run navigation.
- Run inventory, run detail, live-stream baseline, overflow/reconnect semantics, and cancellation.
- Reviews index/detail, related-run navigation, and review-fix dispatch.
- Spec deep links and TechSpec/ADR tab rendering.
- Memory index/detail rendering, opaque `file_id` fetches, and stale-workspace handling.
- Mocked route-state coverage for loading, empty, degraded, partial, and error states.

### Out of Scope

- Executing live browser or daemon scenarios in this planning task.
- Vite-only coverage or any browser flow that bypasses the daemon-served embedded topology.
- In-browser authoring flows; v1 remains operational plus rich-read only.
- Inventing a new browser harness, CLI harness, or mock-only proof path.
- Pixel-perfect/Figma validation; Storybook/MSW is used here for route-state readiness rather than design sign-off.

## Test Strategy and Approach

### QA Lanes

- `E2E`: daemon-served browser flows validated by Playwright against the built binary and seeded workspace fixture.
- `Integration`: route/system/adaptor/storybook tests that validate real request wiring, state transitions, and error handling without a new browser automation layer.
- `Manual-only`: reserved for any human-only verification that cannot be proved through Playwright or integration coverage.
- `Blocked`: a required flow with a concrete blocker in the current repo or UI surface. Blocked is not the same as optional.

### Coverage Matrix

| Flow | Primary Case IDs | Source of Truth | Lane | Automation Status | Existing Harness / Blocker |
|---|---|---|---|---|---|
| Workspace bootstrap, selection, and stale recovery | `TC-FUNC-001` | TechSpec "Active Workspace Model", task `09` | Integration | Existing | `web/src/routes/-app-shell.integration.test.tsx`, `web/src/systems/app-shell/components/app-shell-container.test.tsx`, `web/src/systems/app-shell/hooks/use-active-workspace.test.tsx` |
| Dashboard load and workflow inventory navigation | `TC-FUNC-002` | TechSpec "Route Model" + "Data Flow", task `09`, ADR-002/005 | E2E | Existing | `web/e2e/daemon-ui.smoke.spec.ts` |
| Workflow sync, archive, and workflow drill-down | `TC-FUNC-003` | TechSpec API endpoints, task `09`, task `11`, task `14` | E2E | Existing | `web/e2e/daemon-ui.smoke.spec.ts`, supporting request-shape checks in `web/src/routes/-app-shell.integration.test.tsx` |
| Task board, task detail, and related-run context | `TC-FUNC-004` | TechSpec backend read models, task `11` | E2E | Existing | `web/e2e/daemon-ui.smoke.spec.ts`, `web/src/routes/-workflow-tasks.integration.test.tsx` |
| Reviews index/detail and review-linked run navigation | `TC-FUNC-005` | TechSpec route model, task `12`, ADR-005 | E2E | Existing | `web/e2e/daemon-ui.smoke.spec.ts`, `web/src/routes/-reviews-flow.integration.test.tsx` |
| Review-fix dispatch | `TC-INT-001` | TechSpec `/api/reviews/:slug/rounds/:round/runs`, task `12` | Integration | Existing | `web/src/routes/-reviews-flow.integration.test.tsx`, `web/src/systems/reviews/adapters/reviews-api.test.ts` |
| Spec deep links and document tab rendering | `TC-FUNC-006` | TechSpec document reads, task `12`, ADR-002 | E2E | Existing | `web/e2e/daemon-ui.smoke.spec.ts`, `web/src/routes/-spec-memory-flow.integration.test.tsx` |
| Memory index/detail, opaque file IDs, and stale recovery | `TC-INT-002` | TechSpec "Document Read and Cache Strategy", task `12` | Integration | Existing | `web/src/routes/-spec-memory-flow.integration.test.tsx`, memory adapter/component tests |
| Run inventory, run detail, and live-watch baseline | `TC-FUNC-007` | TechSpec "Streaming Contract", task `10`, task `14` | E2E | Existing | `web/e2e/daemon-ui.smoke.spec.ts`, `web/src/routes/-runs.integration.test.tsx` |
| Run stream overflow and reconnect semantics | `TC-INT-003` | TechSpec "Streaming Contract", task `10` | Integration | Existing | `web/src/routes/-runs.integration.test.tsx`, `web/src/systems/runs/hooks/use-run-stream.test.tsx`, `web/src/systems/runs/lib/stream.test.ts` |
| Run cancel action | `TC-INT-004` | TechSpec `/api/runs/:run_id/cancel`, task `10` | Integration | Existing | `web/src/routes/-runs.integration.test.tsx`, run adapter/component tests |
| Workflow run start from the browser surface | `TC-FUNC-008` | TechSpec `/api/tasks/:slug/runs`, task `10`, task `15` requirement 3 | E2E | Existing | `web/e2e/daemon-ui.smoke.spec.ts`, `web/src/routes/-workflow-tasks.integration.test.tsx`, and `web/src/systems/workflows/components/workflow-inventory-view.test.tsx` protect the browser run-start path |
| Mocked route-state parity (loading, empty, degraded, partial, error) | `TC-INT-005` | ADR-005, task `13` | Integration | Existing | `web/src/storybook/route-stories.test.tsx` and stories under `web/src/routes/_app/stories/` |

## Environment Requirements

| Area | Requirement |
|---|---|
| Repo gate | `make verify` from the current branch state |
| Frontend runtime | Bun `1.3.11` or repo-compatible Bun from `.bun-version` |
| Browser | Playwright Desktop Chrome against the daemon-served embedded SPA |
| Binary | `bin/compozy` built by the repo gate before Playwright runs |
| Seed data | `web/e2e/global.setup.ts` fixture with workflows `daemon`, `daemon-web-ui`, and `archive-ready` |
| Integration runtime | `bunx vitest run --config web/vitest.config.ts ...` in jsdom with generated route tree and typed client |
| Mocked-state harness | Storybook/MSW route stories from `web/src/routes/_app/stories/` |
| Evidence root | `.compozy/tasks/daemon-web-ui/qa/` |
| Execution logs | `task_16` may create `.compozy/tasks/daemon-web-ui/qa/logs/` for browser/daemon command evidence |

## Entry Criteria

- Tasks `08` through `14` remain completed and are still the browser/runtime contract for this feature.
- The QA artifact directories and documents created by `task_15` exist under `.compozy/tasks/daemon-web-ui/qa/`.
- The Playwright daemon-served harness remains wired through `web/playwright.config.ts` and `web/e2e/`.
- The execution branch still uses `make verify` as the canonical repository gate.
- `task_16` reads this plan, the regression suite, the test cases, `_techspec.md`, ADR-002, ADR-005, and tasks `09` through `14` before live execution begins.

## Exit Criteria

- All `P0` cases with an executable harness pass.
- At least 90% of `P1` cases pass, and any failure has either a root-cause fix or a documented issue artifact.
- Any blocked scenario is called out with a specific blocker and no completion language is used for that flow.
- `task_16` writes fresh evidence to `.compozy/tasks/daemon-web-ui/qa/verification-report.md`.
- Any discovered regression is documented under `.compozy/tasks/daemon-web-ui/qa/issues/BUG-*.md` and references the originating `TC-*` case.
- The final post-fix repository gate is a fresh `make verify` pass from the execution branch state.

## Risk Assessment

| Risk | Probability | Impact | Mitigation | Primary Cases |
|---|---|---|---|---|
| Workspace selection and stale recovery are not currently exercised through Playwright because the seeded browser fixture begins with a resolved workspace | Medium | High | Keep workspace bootstrap as a dedicated `P0` integration lane before broader browser smoke | `TC-FUNC-001` |
| Daemon-served browser smoke passes while operational actions like review-fix or cancel regress underneath | Medium | High | Keep action-specific integration cases in the targeted suite and rerun them after any fix | `TC-INT-001`, `TC-INT-004` |
| Stream overflow or reconnect behavior regresses even if the run detail page still loads | Medium | High | Keep reconnect/overflow semantics as a separate integration lane, not implied by smoke | `TC-INT-003` |
| Memory routing regresses back to path-based fetches or stale-workspace handling drifts | Medium | High | Validate opaque `file_id` behavior and stale `412` handling explicitly through the memory flow suite | `TC-INT-002` |
| Route-state harness drifts from the real route tree and stops catching degraded/error branches | Medium | Medium | Keep Storybook/MSW route-state coverage in the full pass and tie it to the same case IDs | `TC-INT-005` |
| Workflow run start can regress if inventory wiring, `detach` mode, or non-conflicting Playwright seed data drift | Medium | High | Keep the browser smoke + route integration coverage in the full pass and preserve the non-conflicting seeded task run in `web/e2e/global.setup.ts` | `TC-FUNC-008` |

## Artifact Layout and Handoff

| Path | Owner in Task 15 | Consumer in Task 16 | Notes |
|---|---|---|---|
| `.compozy/tasks/daemon-web-ui/qa/test-plans/daemon-web-ui-test-plan.md` | Create and keep current | Read before execution | Feature-level QA contract |
| `.compozy/tasks/daemon-web-ui/qa/test-plans/daemon-web-ui-regression.md` | Create and keep current | Execute in listed order | Smoke/targeted/full priority guide |
| `.compozy/tasks/daemon-web-ui/qa/test-cases/TC-*.md` | Create and keep current | Use as execution matrix seed | Stable IDs; do not rename during `task_16` |
| `.compozy/tasks/daemon-web-ui/qa/issues/BUG-*.md` | Directory only in task `15` | Create/update during execution | Must reference the originating case ID |
| `.compozy/tasks/daemon-web-ui/qa/screenshots/` | Directory only in task `15` | Populate during execution | Browser evidence for Playwright or manual reproduction |
| `.compozy/tasks/daemon-web-ui/qa/logs/` | Directory only in task `15` | Optional execution support | Command, daemon, and repro logs when useful |
| `.compozy/tasks/daemon-web-ui/qa/verification-report.md` | Not created in task `15` | Required task `16` output | Fresh evidence only |

## Timeline and Deliverables

1. Planning complete in `task_15`: this test plan, the regression suite, and the traceable `TC-*` set exist under `.compozy/tasks/daemon-web-ui/qa/`.
2. Execution starts in `task_16`: smoke first, then targeted, then full regression using the same artifact paths and case IDs.
3. Bug handling in `task_16`: reproduce, fix at the root, add the narrowest durable regression, rerun the impacted suite, then rerun `make verify`.
4. Final handoff: `.compozy/tasks/daemon-web-ui/qa/verification-report.md`, any `BUG-*` files, and any captured screenshots/logs become the durable QA evidence set for the daemon web UI.
