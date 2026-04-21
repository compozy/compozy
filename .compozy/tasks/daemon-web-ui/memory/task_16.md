# Task Memory: task_16.md

Keep only task-local execution context here. Do not duplicate facts that are obvious from the repository, task file, PRD documents, or git history.

## Objective Snapshot
- Execute the daemon web UI QA matrix from `task_15`, capture fresh browser/API/operator evidence under `.compozy/tasks/daemon-web-ui/qa/`, fix discovered regressions at the root, and finish with a fresh passing `make verify`.

## Important Decisions
- Treat `make verify` as the only completion gate; use daemon-served embedded assets rather than Vite-only checks for browser claims.
- Reclassify `TC-FUNC-008` from blocked to executable E2E once the workflow inventory exposes a real start-run control.
- Use `presentation_mode: "detach"` for browser-started workflow runs.
- Seed the standalone Playwright task run on `daemon` instead of `daemon-web-ui` so the run-start smoke case does not collide with an existing active run.

## Learnings
- Direct root-level vitest invocations with `--config web/vitest.config.ts` can report `No test files found`; the targeted frontend suites must run from `web/` so the config include globs resolve.
- A focused daemon-served Playwright rerun is not authoritative unless `web/dist` and `bin/compozy` were rebuilt first.
- The original run-start smoke failure after adding browser coverage was a harness conflict (`409`) caused by pre-seeding a `daemon-web-ui` task run, not a production daemon rejection of the browser request.

## Files / Surfaces
- `web/src/systems/workflows/components/workflow-inventory-view.tsx`
- `web/src/systems/workflows/components/workflow-inventory-view.test.tsx`
- `web/src/routes/_app/workflows.tsx`
- `web/src/routes/-workflow-tasks.integration.test.tsx`
- `web/e2e/daemon-ui.smoke.spec.ts`
- `web/e2e/global.setup.ts`
- `.compozy/tasks/daemon-web-ui/qa/{logs,screenshots,issues,verification-report.md}`

## Errors / Corrections
- Browser inventory originally had no workflow run-start control; fixed by wiring `useStartWorkflowRun` into `/workflows` and rendering a success banner/link.
- The first browser payload used an unsupported presentation mode and failed server-side; corrected to `detach`.
- The first focused Playwright reruns were misleading because they used stale embedded assets; rebuilt `web/dist` and `bin/compozy` before browser revalidation.
- The initial Playwright run-start smoke then failed with `409` because the harness seeded a conflicting `daemon-web-ui` run; corrected by moving the seeded standalone task run to `daemon`.

## Ready for Next Run
- Fresh QA evidence now exists under `.compozy/tasks/daemon-web-ui/qa/`, including `BUG-001.md`, `verification-report.md`, focused/final logs, and `workflow-start-success.png`.
- Final verification target already passed: `make verify`.
