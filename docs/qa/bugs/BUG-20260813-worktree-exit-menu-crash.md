# BUG-20260813-worktree-exit-menu-crash: Opening Git actions replaces the worktree screen with an error

- **Status:** verified
- **Impact (user-side):** Blocks-Completion
- **Severity:** Critical · **Priority:** P0
- **Persona Affected:** Ada
- **Journey Step:** J-worktree-management assisted exit, step 11
- **Scenarios:** RT-worktree-web-exit-commit-pr
- **Found:** 2026-08-13 · **Report:** docs/qa/reports/2026-08-13-worktree-support.md

## Summary

After committing a worktree, Ada opened Git actions to continue toward a pull request. The entire
worktree surface was replaced by an error, so the assisted-exit journey could not continue in the
Web interface.

## Reproduction

- **Charter:** CH-worktree-fanout-exit-removal · **Tour:** Multi-Tab
- **Environment:** desktop / wifi-fast / en-US, isolated local daemon and Web server

1. Select a ready worktree with a clean committed branch.
2. Open Workspaces overview and the worktree context.
3. Open Git actions.

**Expected:** The daemon-computed exit alternatives open in a menu and the worktree screen remains usable.
**Actual:** The route crashed with `MenuGroupContext is missing` and replaced the worktree screen with an error.

## Evidence

- `/Users/pedronauck/dev/qa-labs/compozy-worktree-support-terminal-rewalk-20260813-150834-409343-lab/qa-artifacts/qa/screenshots/worktree-exit-menu-fixed.png`
- `/Users/pedronauck/dev/qa-labs/compozy-worktree-support-terminal-rewalk-20260813-150834-409343-lab/qa-artifacts/qa/api-worktree-exit-plan.json`

## Fix

- **Root cause:** `DropdownMenuLabel` was rendered without its required Base UI menu group, which throws while the menu mounts.
- **Fix commit:** d7869a8
- **Regression test:** `web/src/systems/workspace/components/__tests__/worktree-exit-control.test.tsx` failed with the same context error before the fix and passes after it.

## Verification

- **Retested:** 2026-08-13, same persona/journey · **Report:** docs/qa/reports/2026-08-13-worktree-support.md
- **Result:** Git actions opened without an error, and the fresh HTTP exit plan exposed the zero-credential browser action for the same worktree.
