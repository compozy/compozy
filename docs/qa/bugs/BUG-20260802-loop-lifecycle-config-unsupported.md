# BUG-20260802-loop-lifecycle-config-unsupported: Loop lifecycle settings cannot be changed through the CLI

- **Status:** fixed
- **Impact (user-side):** Blocks-Completion
- **Severity:** Critical · **Priority:** P0
- **Persona Affected:** Ada
- **Journey Step:** J-tune-loop-lifecycle-defaults, step 1
- **Scenarios:** LP-loop-lifecycle-config-cli
- **Found:** 2026-08-02 · **Report:** docs/qa/reports/2026-08-02-loop-lifecycle-config.md

## Summary

Ada cannot set the newly documented Loop lifecycle defaults through the structured CLI. The
first delivery retry path is rejected as unsupported, so the journey stops before any lifecycle
policy can be saved.

## Reproduction

- **Charter:** CH-loop-lifecycle-config-cli · **Tour:** Feature Tour
- **Environment:** desktop / wifi-fast / en-US; fresh isolated Compozy home and daemon

1. Register an isolated workspace with the running daemon.
2. Run `compozy config set loops.defaults.delivery.retry.max_attempts 4 --scope global --workspace loop-lifecycle-config -o json`.

**Expected:** The CLI accepts the documented path and a fresh `config get` returns `4`.
**Actual:** The command exits non-zero with `config path "loops.defaults.delivery.retry.max_attempts" is not supported by config set`.

## Evidence

- `/Users/pedronauck/dev/qa-labs/compozy-loop-lifecycle-config-20260802-070909-601039-lab/qa-artifacts/qa/observed-results.md`

## Fix

- **Root cause:** The CLI kept its own Loop path-kind registry. The canonical native-tool policy
  contained the 20 lifecycle paths, but the CLI copy did not, so classification failed before
  validation or persistence.
- **Fix commit:** Task 01 checkpoint
- **Regression test:** `TestConfigSetShouldManageLoopLifecycleDefaults` exercises all 20 paths
  through the public CLI and reads each value back from a fresh command.

## Verification

Ada repeated the charter with the rebuilt binary. All 20 writes and reads passed; invalid attempt
and duration values left the last valid values intact; `autopause` remained file-only. The adjacent
JSON/YAML Loop configuration canary also passed.
