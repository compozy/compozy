# BUG-20260826-optional-runtime-run-fails: Orchestrated run fails when category runtimes are left empty

- **Status:** fixed
- **Impact (user-side):** Blocks-Completion
- **Severity:** Critical · **Priority:** P0
- **Persona Affected:** Bruno
- **Journey Step:** J-01 Arrive and use, step 5
- **Scenarios:** LP-implement-tasks-orchestrated-mode
- **Found:** 2026-08-26 · **Report:** `docs/qa/reports/2026-08-26-built-in-loops.md`

## Summary

Bruno could not start the orchestrated delivery path unless every category runtime was supplied,
even though all four runtime controls are optional and should fall through to agent defaults.

## Reproduction

- **Charter:** CH-implement-tasks-orchestrated-mode · **Tour:** Feature Tour
- **Environment:** desktop / wifi-fast / en-US; isolated local daemon with the bundled spec-cycle extension and ACP mock provider

1. Open a workspace containing a valid spec-cycle task graph.
2. Run `implement-tasks` with `mode=orchestrated` and omit `backend_runtime` and `frontend_runtime`.
3. Read the run status through the public CLI after it settles.

**Expected:** The absent runtime selections fall through to the bundled agent and the run reaches its conductor.
**Actual:** The Goal failed during objective materialization with `action_materialization_failed`; no conductor session started.

## Evidence

- `/Users/pedronauck/dev/qa-labs/compozy-built-in-loops-20260826-181651-063490-lab/qa-artifacts/qa/orchestrated-run-failed.json`
- The independently read run status recorded the same failed terminal state after the CLI start returned a run id.

## Fix

- **Root cause:** the Loop omitted absent optional inputs from the render namespace while its Goal objective referenced those keys under strict `missingkey=error` rendering. Each runtime input now has an empty-object definition default, which preserves empty runtime semantics and makes every declared key renderable.
- **Fix commit:** `d2490f96e`
- **Regression test:** `extensions/spec-cycle/embed_test.go` proves omitted runtimes render as `{}`; `internal/daemon/daemon_implement_tasks_e2e_integration_test.go` runs orchestrated mode with all runtime inputs omitted.

## Verification

- **Retested:** pending
- **Result:** pending current-head public-interface replay
