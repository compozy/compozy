# BUG-20260826-halt-rerun-busy: A halted Loop cannot be explicitly rerun

- **Status:** open
- **Impact (user-side):** Blocks-Completion
- **Severity:** Critical · **Priority:** P0
- **Persona Affected:** Bruno
- **Journey Step:** J-configure-and-run-loop, step 4
- **Scenarios:** LP-halt-on-node-failure
- **Found:** 2026-08-26 · **Report:** docs/qa/reports/2026-08-26-loop-issues-fixes.md

## Summary

After choosing `halt`, Bruno sees the run stop exactly once but cannot use the promised explicit
rerun. The command reports that an untouched downstream step is still busy even though the run is
already terminal.

## Reproduction

- **Charter:** CH-loop-effective-config-truth · **Tour:** Feature Tour
- **Environment:** desktop / wifi-fast / en-US; fresh isolated runtime at `http://127.0.0.1:60843`

1. Configure `implement-tasks` with `reattempt_strategy: halt` and run it with a missing task slug.
2. Confirm the first action failure ends the run `failed` in generation 1 with no successor.
3. Run `compozy loop rerun --run-id <id> --from-node load_tasks --request-id qa-explicit-rerun-1`.

**Expected:** One operator-owned rerun generation is admitted from the failed node.
**Actual:** The CLI returns `rerun_busy` because downstream node `collect` remains `pending`.

## Evidence

- `/Users/pedronauck/dev/qa-labs/compozy-loop-issues-fixes-20260826-192350-462800-lab/qa-artifacts/qa/logs/explicit-rerun.json`
- Independent terminal read: `/Users/pedronauck/dev/qa-labs/compozy-loop-issues-fixes-20260826-192350-462800-lab/qa-artifacts/qa/logs/halt-run-status-after-wait.json`

## Fix

- **Root cause:** Pending investigation.
- **Fix commit:** Pending.
- **Regression test:** Pending.

## Verification

- **Retested:** Pending.
- **Result:** Pending.
