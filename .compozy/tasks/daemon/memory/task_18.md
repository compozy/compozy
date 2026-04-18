# Task Memory: task_18.md

Keep only task-local execution context here. Do not duplicate facts that are obvious from the repository, task file, PRD documents, or git history.

## Objective Snapshot
- Generate reusable QA planning artifacts under `.compozy/tasks/daemon/qa/` for daemon bootstrap/recovery, workspace registry, task/review/exec runs, sync/archive, attach/watch, transport parity, `pkg/compozy/runs`, and performance-sensitive regression checks.
- Leave task `19` with stable artifact paths, case IDs, priorities, and automation annotations so execution can start without redefining scope.

## Important Decisions
- Use `qa-output-path=.compozy/tasks/daemon`, which resolves all planning artifacts under `.compozy/tasks/daemon/qa/`.
- Treat existing Go-based CLI/API/integration suites as the automation source of truth; do not invent browser or new E2E frameworks for daemon QA planning.
- Mark browser validation as blocked or out of scope unless a real daemon web surface appears on the execution branch.
- Keep the artifact set stable for `task_19`: `daemon-test-plan.md`, `daemon-regression.md`, stable `TC-*` files, and tracked `issues/` plus `screenshots/` directories.

## Learnings
- The baseline gap is explicit: `.compozy/tasks/daemon/qa/` does not exist yet.
- Repository evidence shows no browser automation harness (`playwright`, `cypress`, `webdriver`, `puppeteer`, `selenium`, `chromedp`) in this branch.
- Existing daemon-critical automation already lives in Go tests across `internal/daemon`, `internal/api/httpapi`, `internal/cli`, `internal/core`, and `pkg/compozy/runs`.
- `make verify` passes cleanly from the final artifact state, so the QA planning docs did not introduce repository verification regressions.

## Files / Surfaces
- `.compozy/tasks/daemon/_techspec.md`
- `.compozy/tasks/daemon/_tasks.md`
- `.compozy/tasks/daemon/task_{12,13,14,15,16,17,18,19}.md`
- `.compozy/tasks/daemon/adrs/adr-{001,002,003,004}.md`
- `.agents/skills/qa-report/references/{test_case_templates.md,regression_testing.md}`
- `Makefile`
- `internal/{daemon,api/httpapi,cli,core}`
- `pkg/compozy/runs`
- `.compozy/tasks/daemon/qa/test-plans/{daemon-test-plan.md,daemon-regression.md}`
- `.compozy/tasks/daemon/qa/test-cases/{TC-FUNC-001.md,TC-FUNC-002.md,TC-FUNC-003.md,TC-FUNC-004.md,TC-FUNC-005.md,TC-FUNC-006.md,TC-INT-001.md,TC-INT-002.md,TC-INT-003.md,TC-PERF-001.md,TC-UI-001.md}`
- `.compozy/tasks/daemon/qa/{issues/.gitkeep,screenshots/.gitkeep}`

## Errors / Corrections
- None.

## Ready for Next Run
- `task_18` produced the daemon QA planning artifact set. Next run (`task_19`) should consume the existing plan/case IDs unchanged, execute smoke first, keep browser validation blocked unless a real web surface appears, and write fresh evidence to `qa/verification-report.md`.
