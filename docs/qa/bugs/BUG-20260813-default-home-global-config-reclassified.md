# BUG-20260813-default-home-global-config-reclassified: Isolated runtime cannot open the operator home workspace

- **Status:** fixed
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Dora
- **Journey Step:** J-worktree-management, entry
- **Scenarios:** MS-worktree-config-bootstrap
- **Found:** 2026-08-13 · **Report:** docs/qa/reports/2026-08-13-worktree-support.md

## Summary

Dora could not start or inspect an isolated runtime while keeping her native provider login because
the daemon and contextual CLI config reads treated the operator's global config as a workspace
overlay and rejected its global-only gateway settings.

## Reproduction

- **Charter:** CH-worktree-bootstrap-hooks · **Tour:** Garbage Tour
- **Environment:** macOS arm64, isolated `COMPOZY_HOME`, operator `HOME`, en-US

1. Bootstrap a fresh isolated runtime home while preserving the operator `HOME` for a native CLI provider.
2. Keep a valid `[gateway]` section in the operator's global `~/.compozy/config.toml`.
3. Start the daemon and let it register the operator home as the default workspace.
4. Run `compozy config list -o json` and `compozy config validate -o json` from the operator home.

**Expected:** The daemon and CLI load the isolated global config and open or inspect the operator
home workspace without reclassifying another global config as a workspace overlay.
**Actual:** Boot or the contextual config read stopped with `gateway settings are global-only`.

## Evidence

- `/Users/pedronauck/dev/qa-labs/compozy-worktree-support-20260813-081758-371939-lab/qa-artifacts/qa/bootstrap-manifest.json`
- `/var/folders/7x/xg204hnd04b81fczcxvjlhzr0000gn/T/compozyqa-5ea84b5e0c62/runtime/logs/compozy.log`
- `/Users/pedronauck/dev/qa-labs/compozy-worktree-support-20260813-083057-155448-lab/qa-artifacts/qa/config-operator-home-list-fixed.json`
- `internal/config/config_test.go` and `internal/daemon/daemon_test.go`
- `internal/cli/config_test.go`

## Fix

- **Root cause:** The workspace resolver always loaded `<root>/.compozy/config.toml`. For the default
  operator-home workspace that path is a user-global config, not a workspace overlay; with an
  isolated runtime home it is also distinct from the active global config, so path deduplication
  alone could not protect it. The daemon and CLI display/management paths each selected overlays
  independently and both needed the same operator-home classification.
- **Fix commit:** `2e741d9d`; `a216668f`
- **Regression test:** `internal/config/config_test.go` — `TestLoadForHomeSkipsDuplicateWorkspaceOverlay`;
  `internal/daemon/daemon_test.go` — `TestBootRegistersOperatorHomeAsDefaultWorkspace`;
  `internal/cli/config_test.go` — `TestConfigCommandsResolveContextBeforeSelectingOverlays`

## Verification

- **Retested:** 2026-08-13 in fresh isolated lab
  `compozy-worktree-support-20260813-083057-155448-lab`.
- **Result:** Passed. The public `compozy status -o json` response reported `daemon.status=running`,
  `user_home_dir=/Users/pedronauck`, and the isolated runtime paths. Public `config list` and the
  config owning suites also pass without loading the operator-global file as an overlay.
- **Evidence:**
  `/Users/pedronauck/dev/qa-labs/compozy-worktree-support-20260813-083057-155448-lab/qa-artifacts/qa/daemon-status-fixed.json`.
  `/Users/pedronauck/dev/qa-labs/compozy-worktree-support-20260813-083057-155448-lab/qa-artifacts/qa/config-operator-home-list-fixed.json`.
