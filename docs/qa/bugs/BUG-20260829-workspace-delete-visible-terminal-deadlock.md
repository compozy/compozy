# BUG-20260829-workspace-delete-visible-terminal-deadlock: Removing a workspace can hang behind a visible exec

- **Status:** verified
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Ada
- **Journey Step:** J-switch-profile-terminal-scope, delete the workspace after terminal work
- **Scenarios:** ET-terminal-profile-segmentation
- **Found:** 2026-08-29 · **Report:** docs/qa/reports/2026-08-28-integrated-terminal-rebase.md

## Summary

Removing a workspace with a long-running `terminal exec --visible` could wait until its command exited.
After the first lifecycle correction, the command closed but its final journal row could not reuse the
already-open workspace database after deletion was staged.

## Reproduction

- **Charter:** CH-terminal-profile-fence · **Tour:** Garbage Tour
- **Environment:** isolated Compozy QA lab / current source binary / CLI + UDS / en-US

1. Register an isolated workspace.
2. Start `terminal exec --visible -- /bin/sh -lc 'sleep 600'` and yield while it is running.
3. Remove that workspace through the CLI.

**Expected:** Removal closes the terminal, persists its final journal row, removes the workspace data,
and returns promptly.

**Actual:** The first attempt waited for the process that only Commit was allowed to close. The next
attempt reached Commit but retried the final row until the request context expired because database
admission was already sealed.

## Root cause and fix

- Visible exec held a general workspace producer lease until process exit, while unregister waited for
  every producer before entering the terminal Commit that closes processes.
- Terminal workspace lifecycle now distinguishes startup producers from registered producers: staging
  waits only for startup, Commit archives terminals, then drains the registered producers before the
  journal database is removed.
- The journal now pins an active lane's workspace database before staging removal. The storage pool may
  reuse only that already-admitted handle during the staged window; it still refuses every new database
  admission and detaches the handle when Commit begins.

## Verification

- Canonical `internal/store/workspacedb`, `internal/terminal/journal`, and `internal/terminal` suites pass
  under `-race` with focused regressions for both lifecycle boundaries.
- A daemon restart recovered the interrupted removal and returned an empty workspace list.
- A fresh visible exec in workspace `ws_d7fcc21c17386bf2` was closed by workspace removal in 0.15 seconds.
  The workspace list was empty, terminal and journal reads returned `workspace not found`, and no command
  process survived.
- Evidence:
  `/Users/pedronauck/dev/qa-labs/compozy-integrated-terminal-profile-retest-20260829-172042-776889-lab/qa-artifacts/qa/profile-segmentation-walk.md`.
