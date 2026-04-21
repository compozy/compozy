Goal (incl. success criteria):

- Execute `task_16` end to end in `/Users/pedronauck/Dev/compozy/_worktrees/daemon-web-ui`: run the daemon web UI QA matrix against the embedded daemon-served UI, capture fresh evidence under `.compozy/tasks/daemon-web-ui/qa/`, fix any root-cause regressions with durable coverage, update workflow memory/tracking, and finish with a fresh passing `make verify`.

Constraints/Assumptions:

- Follow the worktree `AGENTS.md` / `CLAUDE.md`, `task_16.md`, `_techspec.md`, `_tasks.md`, ADR-002, ADR-005, and workflow memory files under `.compozy/tasks/daemon-web-ui/memory/`.
- Required skills for this run: `cy-workflow-memory`, `cy-execute-task`, `qa-execution`, and `cy-final-verify`.
- If QA finds a bug, activate `systematic-debugging`, `no-workarounds`, and `testing-anti-patterns` before changing code or tests.
- Execute in the dedicated `daemon-web-ui` worktree, not the unrelated `agh` checkout.
- The worktree is already dirty in unrelated task-tracking, memory, changelog, and workflow files; do not revert or disturb unrelated edits.
- `make verify` is the mandatory completion gate.

Key decisions:

- Use `.compozy/tasks/daemon-web-ui/qa/` as the single artifact root for logs, screenshots, issues, and the verification report.
- Treat the `task_15` QA plan and regression suite as the execution matrix source of truth.
- Treat the daemon-served Playwright lane as the canonical browser runtime; do not validate core browser claims against Vite-only behavior.
- Use `presentation_mode: "detach"` for browser workflow starts from `/workflows`.
- Seed the standalone Playwright task run on `daemon`, not `daemon-web-ui`, so the browser run-start smoke does not conflict with an already-active run.

State:

- Completed pending final response/cleanup.

Done:

- Read worktree instructions and the required skill files.
- Read shared workflow memory and `task_16` task memory.
- Read `task_16.md`, `_techspec.md`, `_tasks.md`, ADR-001 through ADR-005, and the task_15 QA plan/regression suite/test cases.
- Confirmed the worktree is already dirty in unrelated tracking/docs files and must be handled non-destructively.
- Discovered the repository QA contract and captured the baseline in `.compozy/tasks/daemon-web-ui/qa/logs/01-contract-discovery.log` and `02-baseline-make-verify.log`.
- Executed the smoke and targeted suites with durable logs:
- `03-smoke-workspace-bootstrap.log`
- `04-smoke-playwright.log`
- `05-targeted-review-fix.log`
- `06-targeted-spec-memory.log`
- `07-targeted-runs.log`
- `08-targeted-route-stories.log`
- Fixed the browser workflow run-start regression by wiring `/workflows` to `useStartWorkflowRun`, rendering a success banner/link, correcting the browser request to `presentation_mode: "detach"`, and extending route + Playwright coverage.
- Diagnosed the focused Playwright false negatives:
- stale embedded assets required a rebuild of `web/dist` + `bin/compozy`
- the Playwright harness seeded a conflicting `daemon-web-ui` task run and caused a harness-only `409`
- Rebuilt embedded assets and reran focused validations:
- `13-run-start-regression.log`
- `17-rebuild-embedded-assets.log`
- `23-run-start-playwright-nonconflicting-seed.log`
- Captured fresh browser/operator evidence under `.compozy/tasks/daemon-web-ui/qa/screenshots/`, including `workflow-start-success.png`.
- Ran a fresh final `make verify` successfully, then reran it after task-tracking updates to keep the completion claim aligned with the true final tree. Final gate log: `.compozy/tasks/daemon-web-ui/qa/logs/29-post-tracking-make-verify.log`.

Now:

- Update workflow/task memory, QA artifacts, and task tracking; prepare the final close-out.

Next:

- None.

Open questions (UNCONFIRMED if needed):

- None.

Working set (files/ids/commands):

- `.codex/ledger/2026-04-21-MEMORY-web-ui-qa-exec.md`
- `.compozy/tasks/daemon-web-ui/{task_16.md,_techspec.md,_tasks.md}`
- `.compozy/tasks/daemon-web-ui/adrs/{adr-002.md,adr-005.md}`
- `.compozy/tasks/daemon-web-ui/memory/{MEMORY.md,task_16.md}`
- `.compozy/tasks/daemon-web-ui/qa/test-plans/{daemon-web-ui-test-plan.md,daemon-web-ui-regression.md}`
- `.compozy/tasks/daemon-web-ui/qa/test-cases/TC-*.md`
- `.compozy/tasks/daemon-web-ui/qa/{verification-report.md,issues/BUG-001.md,logs/,screenshots/}`
- `web/e2e/{daemon-ui.smoke.spec.ts,global.setup.ts}`
- `web/src/routes/{_app/workflows.tsx,-workflow-tasks.integration.test.tsx}`
- `web/src/systems/workflows/components/{workflow-inventory-view.tsx,workflow-inventory-view.test.tsx}`
- Key commands:
- `python3 /Users/pedronauck/Dev/compozy/skills/skills/qa-execution/scripts/discover-project-contract.py --root .`
- `bunx vitest run --config vitest.config.ts ...`
- `bunx playwright test --config playwright.config.ts -g 'starts a workflow run from the workflow inventory'`
- `make verify`
