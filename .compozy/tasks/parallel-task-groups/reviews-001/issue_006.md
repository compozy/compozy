---
provider: manual
pr:
round: 1
round_created_at: 2026-07-23T18:09:25Z
status: resolved
file: internal/daemon/task_multi_group_parallel.go
line: 82
severity: low
author: claude-code
provider_ref:
---

# Issue 006: `parked` is classified terminal by the relaunch gate

## Review Comment

The relaunch gate branches on `isTerminalRunStatus(row.Status)`
(`internal/daemon/task_multi_group_parallel.go:82`) to decide between attach and
terminal-report. `isTerminalRunStatus` treats `parked` as terminal, whereas
globaldb's active-run predicate treats it as active (the SQL filters
`status NOT IN ('completed','failed','canceled','crashed')`, which does **not**
include `parked`, in `internal/store/globaldb/registry.go` and `runs.go`).

The two classifiers disagree on where `parked` sits. Failure scenario: a user
parks a group-parallel parent run (a stall that clean-reset recovery could still
resume), then re-issues the same `--parallel-task-groups` command. The gate
takes the terminal branch and returns `parallel_task_groups_selection_terminal …
use --new` instead of re-attaching to the resumable parked run. No data is
corrupted (the fingerprint still prevents a duplicate launch), but the message
is misleading and pushes the user toward `--new` when re-attach would be
correct.

Suggested fix: decide `parked`'s semantics explicitly for this gate. If parked
group-parallel runs are resumable, treat `parked` like an active status here
(attach/return the existing run) rather than routing it through the
terminal-report path; ideally route both this gate and the globaldb active-run
predicate through one shared status classifier so they cannot drift.

## Triage

- Decision: `VALID`
- Root cause: the relaunch gate branched on `isTerminalRunStatus(row.Status)`
  (`task_multi_group_parallel.go:82`). `isTerminalRunStatus`
  (`run_manager.go:3181`) classifies `parked` as terminal — correct for its own
  callers (stall/settlement/integrity bookkeeping), but wrong for the relaunch
  decision. globaldb's active-run predicate
  (`status NOT IN ('completed','failed','canceled','crashed')` in
  `registry.go:1495` and `runs.go:44`) treats `parked` as active. The two
  classifiers disagreed, so a resumable parked parent was routed to the
  terminal-report path and the user was pushed toward `--new` instead of
  re-attaching.
- Fix: added a purpose-specific classifier `isRelaunchSettledRunStatus` in the
  gate's own file that returns terminal only for
  completed/failed/canceled/crashed — byte-for-byte the globaldb active-run list —
  and switched the gate branch to it. `parked` now falls into the
  attach/return-existing branch. `isTerminalRunStatus` is left unchanged so its 15+
  unrelated callers keep their (correct) parked-is-terminal semantics.
- Scope note: a single fully-shared classifier spanning both the daemon gate and
  globaldb's SQL (the reviewer's "ideally") would touch
  `internal/store/globaldb/*` and `run_manager.go`, both outside this batch's
  single in-scope file. The in-scope classifier documents and mirrors the globaldb
  predicate so the two cannot drift silently; unifying them is a follow-up refactor
  beyond this fix's scope.
- Test: added `TestRunManagerTaskMultiGroupParallelParkedSelectionReAttaches` —
  it completes a group-parallel run, drives the persisted parent into `parked`
  via `GetRun`/`UpdateRun`, then re-issues the equivalent selection and asserts the
  gate re-attaches (returns the existing run id) rather than returning a terminal
  problem.
- Verified: the Go gate passes — `make lint` reports `0 issues.`, the full Go
  suite passes with `-race` (`✓ internal/daemon`, including the new parked
  re-attach test), and `make go-build` produces `bin/compozy` (exit 0). The only
  failing `make verify` step is the unrelated `frontend-e2e` Playwright bootstrap,
  which cannot start the daemon inside this sandboxed review worktree
  (`daemon.json: no such file or directory`) — a pre-existing environment
  limitation, not a regression from this change.
