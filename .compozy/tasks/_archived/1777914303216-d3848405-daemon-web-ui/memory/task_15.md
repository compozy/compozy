# Task Memory: task_15.md

Keep only task-local execution context here. Do not duplicate facts that are obvious from the repository, task file, PRD documents, or git history.

## Objective Snapshot
- Create the reusable daemon web UI QA planning artifact set under `.compozy/tasks/daemon-web-ui/qa/` for `task_16`.
- Deliverables: feature test plan, regression suite, traceable `TC-*` cases, and stable `issues/`, `screenshots/`, and `logs/` directories.
- Success requires evidence-based automation classifications and a clean `make verify` pass before tracking is marked complete.

## Important Decisions
- Fixed the `qa-report` root at `.compozy/tasks/daemon-web-ui/qa/` and kept the execution artifact paths identical for `task_16`.
- Classified flows strictly from current repo evidence:
  - Playwright smoke for daemon-served happy-path browser coverage
  - route/system integration tests for workspace bootstrap, review-fix, memory file selection, run cancel, and stream reconnect/overflow
  - Storybook/MSW route stories for degraded/error/loading state parity
- Treated browser workflow-run start as `Blocked`, not manual-only, because the adapter/hook exists but no browser control or route wiring was found in the delivered UI.

## Learnings
- `web/e2e/daemon-ui.smoke.spec.ts` already covers dashboard, workflow inventory, task drill-down, deep-linked spec/memory, reviews-to-runs, and archive against the embedded daemon-served SPA.
- The live Playwright fixture seeds and resolves the workspace before opening the UI, so workspace picker/stale recovery are integration-only QA lanes despite the presence of browser smoke coverage elsewhere.
- `make verify` passed after the QA artifact set was added, including frontend lint/typecheck/test/build, Go fmt/lint/test/build, and the daemon-served Playwright smoke suite.

## Files / Surfaces
- `.compozy/tasks/daemon-web-ui/qa/test-plans/daemon-web-ui-test-plan.md`
- `.compozy/tasks/daemon-web-ui/qa/test-plans/daemon-web-ui-regression.md`
- `.compozy/tasks/daemon-web-ui/qa/test-cases/TC-FUNC-001.md` through `TC-FUNC-008.md`
- `.compozy/tasks/daemon-web-ui/qa/test-cases/TC-INT-001.md` through `TC-INT-005.md`
- `.compozy/tasks/daemon-web-ui/qa/{issues,screenshots,logs}/.gitkeep`
- `web/e2e/daemon-ui.smoke.spec.ts`
- `web/src/routes/{-app-shell,-workflow-tasks,-runs,-reviews-flow,-spec-memory-flow}.integration.test.tsx`
- `web/src/storybook/route-stories.test.tsx`

## Errors / Corrections
- Initial route-test lookup used the wrong filename (`-dashboard-and-workflows.integration.test.tsx`); corrected to the actual app-shell/workflow route integration files before classifying automation.

## Ready for Next Run
- `make verify` is green from the artifact-bearing tree.
- Next action is to keep memory current, update `task_15.md` and `_tasks.md`, and stage only the QA artifact files for commit unless tracking files are explicitly required.
