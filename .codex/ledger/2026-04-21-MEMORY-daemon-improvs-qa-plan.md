Goal (incl. success criteria):

- Complete `task_08.md` by creating reusable QA planning artifacts under `.compozy/tasks/daemon-improvs/analysis/qa/` for the daemon-improvement contract migration, runtime hardening, ACP fault handling, and observability work.
- Success means: the repo contains a feature test plan, execution-ready test cases, and a regression-suite document rooted at `.compozy/tasks/daemon-improvs/analysis/qa/`; every critical flow is classified as `E2E`, `Integration`, `Manual-only`, or `Blocked`; browser validation is explicitly blocked/out of scope; workflow memory and task tracking are updated; `make verify` passes; and one local commit is created.

Constraints/Assumptions:

- Must follow the worktree `AGENTS.md` / `CLAUDE.md`, `cy-workflow-memory`, `cy-execute-task`, `qa-report`, and `cy-final-verify`.
- Scope is documentation and QA planning only. Do not execute daemon flows or pre-emptively fix product bugs in this task.
- The caller requires `qa-output-path=.compozy/tasks/daemon-improvs/analysis`, even though the authoritative TechSpec, ADRs, and task files in this worktree currently live under `.compozy/tasks/daemon-improvs/`.
- Existing worktree contains unrelated task-tracking and ledger changes; do not revert or overwrite them.
- Completion still requires the repository-wide verification gate `make verify` plus tracking updates and a single local commit.

Key decisions:

- Use `.compozy/tasks/daemon-improvs/analysis/qa/` as the only new artifact root for this task so `task_09` can reuse paths unchanged.
- Trust the repository over stale workflow memory: this worktree uses `make verify`; there is no `make test-integration` target, no `web/` directory, and most daemon integration-style tests currently run without `//go:build integration` tags.
- Treat browser validation as blocked/out of scope because the current branch has no daemon web UI surface and no browser automation harness.
- Classify transport parity, client/run-reader compatibility, runtime shutdown, ACP fault handling, and observability against the real current harnesses rather than the originally planned harness package layout.
- Call out missing E2E proof where current coverage is integration-only, especially for live-daemon HTTP/UDS parity and daemon-backed ACP fault surfacing.

State:

- Completed after QA artifact delivery, watcher-race remediation exposed by verification, and a clean repository-wide `make verify`.

Done:

- Read root `AGENTS.md` and `CLAUDE.md` in the `daemon-improvs` worktree.
- Read required skill instructions for `cy-workflow-memory`, `cy-execute-task`, `qa-report`, `qa-execution`, and `cy-final-verify`.
- Read `.compozy/tasks/daemon-improvs/{_techspec.md,_tasks.md,task_03.md,task_04.md,task_05.md,task_06.md,task_07.md,task_08.md,task_09.md}` and ADRs `adr-001.md` through `adr-004.md`.
- Read workflow memory files `.compozy/tasks/daemon-improvs/memory/{MEMORY.md,task_08.md}`.
- Read relevant prior ledgers for daemon QA planning, task-05 runtime hardening, task-07 observability, and the original daemon-improvs task-generation context.
- Confirmed the pre-change signal: `.compozy/tasks/daemon-improvs/analysis/qa/` did not exist before this run.
- Confirmed the current repo reality for planning:
  - `make verify` is the only repository-wide gate in `Makefile`
  - there is no `make test-integration` target
  - no `web/` directory or browser-harness configuration exists
  - daemon/operator integration-style tests exist in `internal/api/httpapi`, `internal/daemon`, `internal/cli`, `internal/core/run/executor`, and `pkg/compozy/runs`
- Created the required QA directory skeleton under `.compozy/tasks/daemon-improvs/analysis/qa/`.
- Wrote the feature test plan at `.compozy/tasks/daemon-improvs/analysis/qa/test-plans/daemon-improvs-analysis-test-plan.md`.
- Wrote the regression suite at `.compozy/tasks/daemon-improvs/analysis/qa/test-plans/daemon-improvs-analysis-regression.md`.
- Wrote the execution-ready test cases under `.compozy/tasks/daemon-improvs/analysis/qa/test-cases/`:
  - `TC-FUNC-001`
  - `TC-FUNC-002`
  - `TC-INT-001`
  - `TC-INT-002`
  - `TC-INT-003`
  - `TC-INT-004`
  - `TC-INT-005`
- Added `.gitkeep` placeholders for `.compozy/tasks/daemon-improvs/analysis/qa/issues/` and `.compozy/tasks/daemon-improvs/analysis/qa/screenshots/`.
- Updated workflow memory with the corrected verification contract and final QA artifact handoff.
- Verification surfaced a real daemon race in `internal/daemon/watchers.go`; fixed `flushPendingChanges` so watch-state reconciliation happens before sync when rename/delete changes the watch set.
- Re-ran `go test ./internal/daemon -count=10` successfully after the fix.
- Re-ran `make verify` successfully after the fix:
  - `fmt`: passed
  - `lint`: passed with `0 issues`
  - `test`: passed with `DONE 2530 tests, 2 skipped`
  - `build`: passed

Now:

- Update task tracking files from the verified state and create the scoped local commit.

Next:

- None after tracking updates and the required local commit.

Open questions (UNCONFIRMED if needed):

- None currently blocking.

Working set (files/ids/commands):

- `.codex/ledger/2026-04-21-MEMORY-daemon-improvs-qa-plan.md`
- `.compozy/tasks/daemon-improvs/{_techspec.md,_tasks.md,task_03.md,task_04.md,task_05.md,task_06.md,task_07.md,task_08.md,task_09.md}`
- `.compozy/tasks/daemon-improvs/adrs/{adr-001.md,adr-002.md,adr-003.md,adr-004.md}`
- `.compozy/tasks/daemon-improvs/memory/{MEMORY.md,task_08.md}`
- `.compozy/tasks/daemon-improvs/analysis/qa/{test-plans,test-cases,issues,screenshots}`
- `internal/daemon/watchers.go`
- `.compozy/tasks/daemon/qa/` (reference only)
- Commands: `rg`, `sed`, `find`, `git status --short`, `go test ./internal/daemon -count=10`, `make verify`
