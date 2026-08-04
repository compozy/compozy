# BUG-20260802-node-kill-leaves-run-live: Killing a waiting node left its Run live forever

- **Status:** verified
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Ada
- **Journey Step:** J-07 operate a Loop through native lifecycle controls, step 5
- **Scenarios:** LP-agent-operates-lifecycle-via-native-tools; TA-076
- **Found:** 2026-08-02 · **Report:** docs/qa/reports/2026-08-02-loop-node-lifecycle-task07.md

## Summary

Ada killed a waiting node and received a successful structured result. The node and wait closed, but
the parent Run remained `running` because no coordinator was awakened to reconcile the graph.

## Reproduction

- **Charter:** CH-agent-loop-lifecycle-native · **Tour:** Feature Tour
- **Environment:** isolated macOS lab / native tools and fresh CLI reads / en-US

1. Start `lifecycle-control` and confirm `hold` is `waiting`.
2. Invoke `compozy__loop_node_kill` for `hold`.
3. Fresh-read the Run and node inventory.

**Expected:** the immediate kill closes the node and wakes Run reconciliation.
**Actual:** Run `looprun-f3f39e2217c8b30c` retained live Run truth after `hold` was canceled.

## Evidence

- Isolated lab manifest:
  `/Users/pedronauck/dev/qa-labs/compozy-loop-native-lifecycle-20260802-235403-385288-lab/qa-artifacts/qa/bootstrap-manifest.json`
- Candidate Run `looprun-f3f39e2217c8b30c`; repaired replay Run
  `looprun-a7626a6f4ef766f7`.

## Fix

- **Root cause:** immediate node kill committed terminal node truth but returned no reserved
  coordinator to the service, so the activation path had nothing to start.
- **Fix commit:** Task 07 checkpoint
- **Regression test:** kill cases in
  `TestGlobalDBLoopNodeCancellationShouldCloseAttemptsAndEffectsAtomically` and
  `TestServiceCancellationShouldRecordCanceledTerminalTruth`.

## Verification

- **Retested:** 2026-08-02, same persona/journey · **Report:**
  `docs/qa/reports/2026-08-02-loop-node-lifecycle-task07.md`
- **Result:** Pass. Run `looprun-a7626a6f4ef766f7` reconciled to terminal `failed` after the
  required `hold` node was killed, and fresh-read `waits: []`.
