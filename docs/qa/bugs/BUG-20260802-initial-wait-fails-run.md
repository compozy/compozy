# BUG-20260802-initial-wait-fails-run: A Loop failed as soon as its first node began waiting

- **Status:** verified
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Ada
- **Journey Step:** J-07 operate a Loop through native lifecycle controls, step 2
- **Scenarios:** LP-agent-operates-lifecycle-via-native-tools; TA-076
- **Found:** 2026-08-02 · **Report:** docs/qa/reports/2026-08-02-loop-node-lifecycle-task07.md

## Summary

Ada started a valid Loop whose first control node waits for one hour. The wait and its inventory row
were created, but the Run immediately became `failed`, so no lifecycle tool could manage the live
wait it had just exposed.

## Reproduction

- **Charter:** CH-agent-loop-lifecycle-native · **Tour:** Feature Tour
- **Environment:** isolated macOS lab / native tools and CLI / en-US

1. Publish the `lifecycle-control` fixture with `source -> wait -> transform`.
2. Start it through the public CLI and read the Run through `compozy loop status`.
3. Compare the Run status with the `hold` output and durable wait inventory.

**Expected:** the Run remains `running` while `hold` is `waiting`.
**Actual:** Run `looprun-0e28ce82f3a779e3` became `failed` while `hold` and its wait were live.

## Evidence

- Isolated lab manifest:
  `/Users/pedronauck/dev/qa-labs/compozy-loop-native-lifecycle-20260802-235403-385288-lab/qa-artifacts/qa/bootstrap-manifest.json`
- Candidate Run `looprun-0e28ce82f3a779e3`; repaired replay Run `looprun-cab69e8c2333002e`.

## Fix

- **Root cause:** the initial coordinator plan treated waiting outputs as if no work remained, so
  the no-ready-nodes boundary failed the Run after successfully parking the wait.
- **Fix commit:** Task 07 checkpoint
- **Regression test:** `internal/loop/coordinator_control_test.go`, canonical
  `TestCoordinatorRunnerShouldParkWaitControls` generation-zero case.

## Verification

- **Retested:** 2026-08-02, same persona/journey · **Report:**
  `docs/qa/reports/2026-08-02-loop-node-lifecycle-task07.md`
- **Result:** Pass. Fresh Run `looprun-cab69e8c2333002e` stayed `running` with one waiting node,
  then reached `done` after the native resume supplied the wait payload.
