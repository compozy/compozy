# BUG-20260802-parked-node-cancel-stalls: Canceling a waiting node could do nothing, stall, or retain its wait

- **Status:** verified
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Ada
- **Journey Step:** J-07 operate a Loop through native lifecycle controls, step 4
- **Scenarios:** LP-agent-operates-lifecycle-via-native-tools; TA-076
- **Found:** 2026-08-02 · **Report:** docs/qa/reports/2026-08-02-loop-node-lifecycle-task07.md

## Summary

Ada canceled the live `hold` node through `compozy__loop_node_cancel`. Depending on the replay, the
tool reported success without changing the node, left cancellation at `draining`, or marked the node
`canceled` while continuing to list its wait as active. A managing agent could not know whether the
control had really won.

## Reproduction

- **Charter:** CH-agent-loop-lifecycle-native · **Tour:** Feature Tour
- **Environment:** isolated macOS lab / native tools and fresh CLI reads / en-US

1. Start `lifecycle-control` and wait until `hold` is `waiting`.
2. Invoke `compozy__loop_node_cancel` for `hold`.
3. Fresh-read the Run, node controls, task-run coordinators, and active wait inventory.

**Expected:** the node converges to `canceled`, its wait disappears from active inventory, and a
coordinator reconciles the remaining graph.
**Actual:** Run `looprun-d03de69a0a414499` first stayed waiting, then stalled at `draining`; a later
candidate replay canceled the node but retained the active wait.

## Evidence

- Isolated lab manifest:
  `/Users/pedronauck/dev/qa-labs/compozy-loop-native-lifecycle-20260802-235403-385288-lab/qa-artifacts/qa/bootstrap-manifest.json`
- Candidate Run `looprun-d03de69a0a414499`; final repaired replay Run
  `looprun-86df5fa096c0e4d6`.

## Fix

- **Root cause:** waiting and paused outputs were absent from the cancellation live/drained sets;
  the generic wake reused a generation-scoped identity that could collide; and the coordinator's
  snapshot terminalization did not claim the canceled node's active waits.
- **Fix commit:** Task 07 checkpoint
- **Regression test:** canonical cancellation suites in
  `internal/store/globaldb/global_db_loop_test.go`,
  `internal/loop/coordinator_lifecycle_test.go`, and
  `internal/loop/coordinator_control_test.go`.

## Verification

- **Retested:** 2026-08-02, same persona/journey · **Report:**
  `docs/qa/reports/2026-08-02-loop-node-lifecycle-task07.md`
- **Result:** Pass. Run `looprun-86df5fa096c0e4d6` returned node control revision 4 with native
  provenance, terminalized `hold`, and fresh-read `waits: []`.
