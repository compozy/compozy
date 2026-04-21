Goal (incl. success criteria):

- Complete `task_15` by creating reusable daemon web UI QA planning artifacts under `.compozy/tasks/daemon-web-ui/qa/`.
- Success means: the repo contains a feature-level QA plan, traceable `TC-*` cases, and a regression suite rooted at the fixed `qa-output-path`; each critical browser/operator flow is classified as `E2E`, `Integration`, `Manual-only`, or `Blocked` from real repository evidence; workflow memory and task tracking are updated after verification; and `make verify` passes before the local commit.

Constraints/Assumptions:

- Must follow the worktree `AGENTS.md` / `CLAUDE.md`, task `15`, `_techspec.md`, `_tasks.md`, ADR-002, ADR-005, tasks `08` through `14`, and the caller-provided workflow memory files.
- Required skills for this run: `cy-workflow-memory`, `cy-execute-task`, `qa-report`, and `cy-final-verify`.
- This is a bounded QA-planning task, so execution stays focused on documentation and traceability rather than live browser/daemon validation.
- The worktree is already dirty with unrelated task-tracking, memory, changelog, and workflow-file changes; do not revert or disturb them.
- Auto-commit is enabled, but only after fresh `make verify`, self-review, and task-local tracking updates.

Key decisions:

- Root every artifact under `.compozy/tasks/daemon-web-ui/qa/` so `task_16` can consume paths unchanged.
- Classify automation strictly from repository evidence: Playwright smoke specs count as `E2E`, route/system tests and Storybook/MSW count as `Integration`, and any flow without a browser/operator entrypoint gets an explicit blocker instead of being hand-waved to manual.
- Treat the embedded daemon-served topology from `task_08` and `task_14` as the canonical browser lane; do not plan against Vite-only behavior.

State:

- Verified and ready to hand off. QA artifacts are staged; memory and tracking artifacts remain intentionally unstaged.

Done:

- Read worktree instructions, required skill guides, shared workflow memory, and task memory.
- Scanned relevant ledgers for cross-task QA and daemon-web-ui context.
- Read `task_15`, `_tasks.md`, `_techspec.md`, ADR-002, ADR-005, and tasks `08` through `14`.
- Confirmed the pre-change gap: `.compozy/tasks/daemon-web-ui/qa/` does not exist yet.
- Confirmed the repository QA contract and automation seams:
  - `make verify` is the full repo gate
  - `web/e2e/daemon-ui.smoke.spec.ts` covers daemon-served dashboard/workflow/task, spec/memory, reviews-to-runs, and archive smoke flows
  - route/system integration tests cover workspace bootstrap/stale recovery, task/review/spec/memory routes, run cancel, and run-stream reconnect/overflow behavior
  - no browser workflow-run start control is currently wired in the route tree even though the adapter/hook exists
- Created `.compozy/tasks/daemon-web-ui/qa/test-plans/daemon-web-ui-test-plan.md`.
- Created `.compozy/tasks/daemon-web-ui/qa/test-plans/daemon-web-ui-regression.md`.
- Created execution-ready QA cases under `.compozy/tasks/daemon-web-ui/qa/test-cases/` covering workspace bootstrap, dashboard/workflows, tasks, reviews, review-fix, spec, memory, runs, reconnect/cancel, route states, and the blocked browser run-start gap.
- Added `.gitkeep` placeholders for `.compozy/tasks/daemon-web-ui/qa/{issues,screenshots,logs}/`.
- Ran `make verify` successfully after the QA artifact set was in place.
- Updated workflow memory, task-local memory, `task_15.md`, and `_tasks.md` on disk.
- Staged only the QA artifact files under `.compozy/tasks/daemon-web-ui/qa/` for the local commit, leaving tracking/memory files unstaged per task rules.

Now:

- Create the required local commit from the staged QA artifact set.

Next:

- Final response with verification evidence and the note that tracking/memory files were updated but left unstaged intentionally.

Open questions (UNCONFIRMED if needed):

- None.

Working set (files/ids/commands):

- `.codex/ledger/2026-04-21-MEMORY-web-ui-qa.md`
- `.compozy/tasks/daemon-web-ui/{_techspec.md,_tasks.md,task_08.md,task_09.md,task_10.md,task_11.md,task_12.md,task_13.md,task_14.md,task_15.md,task_16.md}`
- `.compozy/tasks/daemon-web-ui/adrs/{adr-002.md,adr-005.md}`
- `.compozy/tasks/daemon-web-ui/memory/{MEMORY.md,task_15.md}`
- `.agents/skills/{cy-workflow-memory,cy-execute-task,cy-final-verify,qa-report,qa-execution}/...`
- `web/e2e/{daemon-ui.smoke.spec.ts,global.setup.ts,support/daemon-fixture.ts}`
- `web/src/routes/{-app-shell.integration.test.tsx,-workflow-tasks.integration.test.tsx,-runs.integration.test.tsx,-reviews-flow.integration.test.tsx,-spec-memory-flow.integration.test.tsx}`
- Commands: `sed -n`, `find`, `rg`, `git status --short`
