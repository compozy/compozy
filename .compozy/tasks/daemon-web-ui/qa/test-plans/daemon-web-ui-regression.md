# Daemon Web UI Regression Suite

## Purpose

This suite turns the daemon web UI QA plan into an execution order for `task_16`. It keeps one artifact root, one case-ID vocabulary, and one final repository gate: `make verify`.

## Execution Rules

1. Run the smoke suite first. If any executable `P0` case fails, stop and fix before continuing.
2. Run the targeted suite next for action depth, stale/error branches, and route-state harness coverage.
3. Run the full suite before closing `task_16`, including a fresh `make verify`.
4. After any bug fix, rerun the narrow failing case, then the affected suite, then the full gate.
5. Do not downgrade browser action flows without a concrete blocker. `TC-FUNC-008` is now executable E2E and must stay in the targeted/final rerun set.

## Pass / Fail Criteria

### PASS

- All executable `P0` cases pass.
- At least 90% of executable `P1` cases pass.
- No critical browser/operator bug remains open.
- `make verify` passes after the last code change.

### FAIL

- Any executable `P0` case fails.
- A browser/operator regression is found with no root-cause fix or accepted blocker.
- `make verify` fails after the final fix set.

### CONDITIONAL

- Only `P1` failures remain, each has a documented workaround or fix plan, and no blocked flow is being misrepresented as validated.

## Priority Bands

- `P0`: workspace bootstrap/stale recovery, dashboard/workflow inventory, sync/archive, task board/detail, reviews detail/run navigation, run inventory/detail baseline.
- `P1`: review-fix dispatch, spec deep links, memory opaque-file behavior, run cancel, run reconnect/overflow semantics, route-state parity, workflow run-start blocker resolution.
- `P2`: supplemental manual or responsive checks added during execution only if the branch changes warrant them.

## Smoke Suite

**Goal:** prove the daemon-served operator console is healthy enough for deeper validation.

**Expected duration:** 15-30 minutes.

**Stop condition:** any executable `P0` failure blocks the rest of the suite.

| Order | Case ID(s) | Priority | Flow | Lane | Command / Spec |
|---|---|---|---|---|---|
| 1 | `TC-FUNC-001` | P0 | Workspace bootstrap, selection, and stale recovery | Integration | `bunx vitest run --config web/vitest.config.ts web/src/routes/-app-shell.integration.test.tsx web/src/systems/app-shell/components/app-shell-container.test.tsx web/src/systems/app-shell/hooks/use-active-workspace.test.tsx` |
| 2 | `TC-FUNC-002`, `TC-FUNC-003`, `TC-FUNC-004`, `TC-FUNC-005`, `TC-FUNC-007` | P0 | Daemon-served dashboard, workflows, tasks, reviews, runs, and archive smoke pack | E2E | `bun run frontend:e2e` (`web/e2e/daemon-ui.smoke.spec.ts`) |

## Targeted Suite

**Goal:** validate the action-heavy and state-heavy surfaces most likely to regress after fixes or follow-up work.

**Expected duration:** 30-60 minutes.

| Order | Case ID(s) | Priority | Flow | Lane | Command / Spec |
|---|---|---|---|---|---|
| 1 | `TC-INT-001` | P1 | Review-fix dispatch | Integration | `bunx vitest run --config web/vitest.config.ts web/src/routes/-reviews-flow.integration.test.tsx web/src/systems/reviews/adapters/reviews-api.test.ts` |
| 2 | `TC-FUNC-006`, `TC-INT-002` | P1 | Spec deep links, memory index/detail, opaque file IDs, stale recovery | E2E + Integration | `bunx vitest run --config web/vitest.config.ts web/src/routes/-spec-memory-flow.integration.test.tsx web/src/systems/memory/adapters/memory-api.test.ts web/src/systems/memory/components/workflow-memory-view.test.tsx` plus spec checkpoints from `bun run frontend:e2e` if the document surfaces changed |
| 3 | `TC-INT-003`, `TC-INT-004` | P1 | Run reconnect/overflow semantics and cancel action | Integration | `bunx vitest run --config web/vitest.config.ts web/src/routes/-runs.integration.test.tsx web/src/systems/runs/hooks/use-run-stream.test.tsx web/src/systems/runs/lib/stream.test.ts web/src/systems/runs/adapters/runs-api.test.ts web/src/systems/runs/components/run-detail-view.test.tsx` |
| 4 | `TC-INT-005` | P1 | Mocked route-state parity across loading, empty, degraded, partial, and error branches | Integration | `bunx vitest run --config web/vitest.config.ts web/src/storybook/route-stories.test.tsx` |
| 5 | `TC-FUNC-008` | P1 | Workflow run start from the browser surface | E2E + Integration | `bunx playwright test --config playwright.config.ts -g 'starts a workflow run from the workflow inventory'` plus `bunx vitest run --config web/vitest.config.ts web/src/routes/-workflow-tasks.integration.test.tsx web/src/systems/workflows/components/workflow-inventory-view.test.tsx` |

## Full Suite

**Goal:** release-level validation for the daemon web UI branch.

**Expected duration:** 60+ minutes depending on bug-fix churn.

| Order | Scope | Required Cases / Commands |
|---|---|---|
| 1 | Smoke prerequisite | Run every smoke-suite row in listed order |
| 2 | Targeted follow-up | Run every targeted-suite row that is executable on the branch |
| 3 | Repository gate | `make verify` |
| 4 | Post-fix rerun | Re-run the narrow failing case(s), then the affected smoke/targeted row(s), then `make verify` again |

## Explicit P0 / P1 Mapping

| Case ID | Priority | Notes |
|---|---|---|
| `TC-FUNC-001` | P0 | Workspace context is a prerequisite for every browser flow |
| `TC-FUNC-002` | P0 | Dashboard and workflow inventory are the main operator entrypoint |
| `TC-FUNC-003` | P0 | Sync/archive are core operational actions on the workflow surface |
| `TC-FUNC-004` | P0 | Task inspection is a primary workflow drill-down path |
| `TC-FUNC-005` | P0 | Review detail and run linkage are operator-critical |
| `TC-FUNC-007` | P0 | Run inventory/detail are the main runtime control surface |
| `TC-INT-001` | P1 | Review-fix dispatch is important, but sits below base review navigation in smoke order |
| `TC-FUNC-006` | P1 | Spec access is important for operator context, but not the first smoke blocker |
| `TC-INT-002` | P1 | Memory opaque-file behavior is critical enough for targeted coverage |
| `TC-INT-003` | P1 | Reconnect/overflow semantics matter after baseline run visibility is proven |
| `TC-INT-004` | P1 | Cancel behavior is an operational action validated after smoke |
| `TC-FUNC-008` | P1 | Browser operator start-run flow with dedicated smoke and integration coverage |
| `TC-INT-005` | P1 | Route-state parity protects degraded/error branches not seeded by the live harness |

## Blocked and Manual-Only Notes

- No critical daemon web UI flow remains blocked after `task_16`.
- No critical flow currently requires a `Manual-only` label. Supplemental browser screenshots can still be captured when the execution report benefits from them.

## Evidence Output

- Record executed commands, verdicts, blockers, and post-fix reruns in `.compozy/tasks/daemon-web-ui/qa/verification-report.md`.
- Write discovered regressions to `.compozy/tasks/daemon-web-ui/qa/issues/BUG-*.md` and include the originating case ID.
- Store browser screenshots under `.compozy/tasks/daemon-web-ui/qa/screenshots/`.
- Store command or daemon logs under `.compozy/tasks/daemon-web-ui/qa/logs/` when they materially help reproduction or handoff.
