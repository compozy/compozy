Goal (incl. success criteria):

- Complete daemon task `18` by creating reusable QA planning artifacts under `.compozy/tasks/daemon/qa/` for daemon bootstrap/recovery, workspace registry, task/review/exec runs, sync/archive, attach/watch, transport parity, and `pkg/compozy/runs`.
- Success means: the repo contains a feature test plan, execution-ready test cases, and regression-suite definitions rooted at `.compozy/tasks/daemon/qa/`; artifacts trace back to the TechSpec/ADRs/tasks; browser coverage is explicitly blocked or out of scope if no real daemon web surface exists; task/workflow memory and task tracking are updated; and `make verify` passes before commit.

Constraints/Assumptions:

- Must follow `AGENTS.md`, `CLAUDE.md`, task `18`, `_techspec.md`, `_tasks.md`, ADR-001 through ADR-004, and the provided workflow memory files.
- Required skills for this run: `cy-workflow-memory`, `cy-execute-task`, `qa-report`, and `cy-final-verify`.
- Skip `brainstorming`: this is a bounded QA-planning task against an approved daemon design, not a new feature design exercise.
- Existing worktree has unrelated task-tracking and ledger changes; do not revert or disturb them.
- Completion requires fresh `make verify`, self-review, tracking updates, and one local commit.

Key decisions:

- Root every artifact under `.compozy/tasks/daemon/qa/` so task `19` can consume paths unchanged.
- Classify automation strictly from repository evidence: existing Go/CLI/API integration suites count, absent browser harness means no invented browser automation lane.
- Use the repo's real verification contract and named daemon regression suites as the canonical automation references in the QA artifacts.

State:

- In progress after clean verification; only task tracking updates and the required local commit remain.

Done:

- Read required skill guides for workflow memory, task execution, final verification, and QA planning.
- Read repository instructions (`AGENTS.md`, `CLAUDE.md`), shared workflow memory, and task memory.
- Scanned related daemon ledgers for tasks 14-17 plus daemon QA task creation context.
- Read daemon `_techspec.md`, `_tasks.md`, ADR-001/002/003/004, and task docs `12` through `17` plus `19`.
- Confirmed the pre-change gap: `.compozy/tasks/daemon/qa/` does not exist yet.
- Confirmed no browser/E2E web harness exists in the repo (`rg --files` found no Playwright/Cypress/WebDriver/etc. configs), so browser validation must be documented as blocked/out of scope.
- Identified concrete existing daemon automation seams in Go tests across `internal/daemon`, `internal/api/httpapi`, `internal/cli`, `internal/core`, and `pkg/compozy/runs`.
- Created `.compozy/tasks/daemon/qa/test-plans/daemon-test-plan.md`.
- Created `.compozy/tasks/daemon/qa/test-plans/daemon-regression.md`.
- Created execution-ready daemon test cases under `.compozy/tasks/daemon/qa/test-cases/` for bootstrap/recovery, workspace registry, task runs, sync/archive, review, exec, attach/watch, transport parity, public run readers, performance, and manual TUI confirmation.
- Added `.gitkeep` placeholders for `qa/issues/` and `qa/screenshots/` so task `19` inherits the full artifact layout.
- Ran `make verify` successfully from the final artifact state.

Now:

- Update `task_18.md` and `_tasks.md` from the verified state, then create the required local commit while respecting the repo rule about tracking-only files.

Next:

- Decide the final staging set, excluding unrelated dirty files.
- Create one local commit after tracking updates.

Open questions (UNCONFIRMED if needed):

- None.

Working set (files/ids/commands):

- `.codex/ledger/2026-04-18-MEMORY-daemon-qa-plan.md`
- `.compozy/tasks/daemon/{_techspec.md,_tasks.md,task_12.md,task_13.md,task_14.md,task_15.md,task_16.md,task_17.md,task_18.md,task_19.md}`
- `.compozy/tasks/daemon/adrs/{adr-001.md,adr-002.md,adr-003.md,adr-004.md}`
- `.compozy/tasks/daemon/memory/{MEMORY.md,task_18.md}`
- `.agents/skills/{qa-report,qa-execution,cy-workflow-memory,cy-execute-task,cy-final-verify}/...`
- `Makefile`
- `internal/{daemon,api/httpapi,cli,core}`
- `pkg/compozy/runs`
- Commands: `rg`, `sed -n`, `find .compozy/tasks/daemon/qa`, `git status --short`
