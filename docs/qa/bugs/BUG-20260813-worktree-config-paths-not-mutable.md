# BUG-20260813-worktree-config-paths-not-mutable: Public config mutation rejects worktree keys

- **Status:** fixed
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Dora
- **Journey Step:** J-worktree-management, configuration
- **Scenarios:** MS-worktree-config-bootstrap
- **Found:** 2026-08-13 · **Report:** docs/qa/reports/2026-08-13-worktree-support.md

## Summary

Dora could read the `[worktrees]` lifecycle settings but neither the CLI nor native config tool
could set or unset them, leaving the feature without its required agent-manageable configuration
surface.

## Reproduction

- **Charter:** CH-worktree-bootstrap-hooks · **Tour:** Feature Tour
- **Environment:** macOS arm64, isolated runtime, en-US

1. Start a workspace through the public daemon.
2. Run `compozy config set worktrees.setup_command 'sleep 20' --scope workspace -o json`.

**Expected:** The typed setting is validated, written to the workspace overlay, and applied live.
**Actual:** The command returned `unsupported config path "worktrees.setup_command"`.

## Evidence

- `/Users/pedronauck/dev/qa-labs/compozy-worktree-support-20260813-083057-155448-lab/qa-artifacts/qa/config-worktree-slow-setup-fixed.json`
- `/Users/pedronauck/dev/qa-labs/compozy-worktree-support-20260813-083057-155448-lab/qa-artifacts/qa/config-worktree-slow-setup-unset.json`

## Fix

- **Root cause:** `WorktreesConfig` was present in the config schema and validators but its six
  scalar paths were omitted from the shared CLI/native mutable-path policy.
- **Fix commit:** `a216668f`
- **Regression tests:** `internal/config/tool_surface_test.go` — `TestToolConfigPathPolicy`;
  `internal/cli/config_test.go` — `TestConfigSetSupportsWorktreeLifecyclePaths`

## Verification

- **Retested:** 2026-08-13 in
  `compozy-worktree-support-20260813-083057-155448-lab`.
- **Result:** Passed. Public `config set` applied the workspace setup command live, a browser-created
  worktree entered setup, cancellation removed the pending checkout and branch, and public
  `config unset` restored the default.
- **Evidence:**
  `/Users/pedronauck/dev/qa-labs/compozy-worktree-support-20260813-083057-155448-lab/qa-artifacts/qa/browser-worktree-cancel-pending.png`;
  `/Users/pedronauck/dev/qa-labs/compozy-worktree-support-20260813-083057-155448-lab/qa-artifacts/qa/browser-worktree-cancel-complete.json`.
