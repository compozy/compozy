# QA Run Report — 2026-08-02 — Loop node lifecycle Task 07

- **Scope:** Task 07 adds public Loop run and node lifecycle controls across HTTP, UDS, CLI,
  native tools, hooks, and the run page.
- **Cadence tier:** targeted
- **Build:** working tree after Task 06 checkpoint `de967880` · **Environment:** fresh isolated
  lab, manifest `/Users/pedronauck/dev/qa-labs/compozy-loop-native-lifecycle-20260802-235403-385288-lab/qa-artifacts/qa/bootstrap-manifest.json`
- **Started:** 2026-08-03T02:54:03Z · **Status:** pass after remediation

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Ada | Power User | desktop / wifi-fast / en-US | CH-agent-loop-lifecycle-native |
| Bruno | Power User | desktop / wifi-fast / en-US | CH-loop-run-cancel-control |

## Flows in Scope

- `J-07` — operate a Loop through agent-facing native lifecycle controls
  (`../journeys/J-07-agent-operated-run.md`)
- `J-04` — inspect and control a live Loop run from the web surface
  (`../journeys/J-04-run-loop.md`)

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-agent-loop-lifecycle-native | J-07 / LP-agent-operates-lifecycle-via-native-tools; TA-076 | Ada | Feature Tour | Fixed | BUG-20260802-initial-wait-fails-run; BUG-20260802-parked-node-cancel-stalls; BUG-20260802-node-kill-leaves-run-live | Task 07 checkpoint |
| 2 | CH-loop-run-cancel-control | J-04 / TA-084 | Bruno | Feature Tour | Pass | | |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

### CH-agent-loop-lifecycle-native

- Discovered the exact eight lifecycle descriptors and confirmed `compozy__loop_stop` was unknown.
- Exercised native inventory, node pause/resume/cancel/kill/requeue, and Run cancel/kill. Fresh CLI
  reads independently confirmed each successful mutation.
- Repeated requeue on active waiting Run `looprun-e7c1de6ea89552c1`; both attempts returned
  `node_not_quarantined`, `actual_state=active`, and
  `allowed_transitions=pause,cancel,kill`.
- Workspace-isolation Run `looprun-48465e2e2366e555` appeared once in the owner inventory and zero
  times in workspace `pedronauck`; a foreign node mutation was hidden as not found and left the
  owner Run waiting.
- Three blocking lifecycle defects appeared during the walk. Each received a canonical regression,
  a production fix, and a same-persona public replay before the charter continued.

### CH-loop-run-cancel-control

- Opened live waiting Run `looprun-009750d54a610b7e` from its deep link using the built assets from
  this worktree.
- Confirmed the visible control was `Cancel run` and no Stop control existed, then clicked Cancel
  once and observed `Canceled` immediately.
- Reloaded the deep link, navigated to Runs, and returned through browser history. The page remained
  readable and durably `Canceled`.
- Captures:
  `/Users/pedronauck/dev/qa-labs/compozy-loop-native-lifecycle-20260802-235403-385288-lab/qa-artifacts/qa/evidence/loop-run-cancel-control.png`
  and
  `/Users/pedronauck/dev/qa-labs/compozy-loop-native-lifecycle-20260802-235403-385288-lab/qa-artifacts/qa/evidence/loop-run-canceled-after-action.png`.

## What Was Fixed

- `BUG-20260802-initial-wait-fails-run` — the initial coordinator now treats a waiting output as
  in-flight instead of failing through the no-ready-nodes boundary.
- `BUG-20260802-parked-node-cancel-stalls` — waiting/paused nodes are live cancellation targets,
  cancellation reserves a collision-free coordinator wake, and terminalization claims active waits
  in the same transaction.
- `BUG-20260802-node-kill-leaves-run-live` — immediate node kill now reserves and activates the exact
  coordinator that reconciles the remaining graph.

## Paper Cuts

None observed so far.

## Runtime Errors Observed

No unhandled browser or daemon errors remained after the repaired replays. The deliberate native
schema rejection for an extra Run-cancel `reason` field and the repeated non-quarantined requeue were
expected structured errors.

## Human Verifications Needed

None identified so far.

## Decisions for a Human

None identified so far.

## Learnings

- A parked wait is live execution state. Cancellation and coordinator planning must classify it the
  same way on request, drain, terminalization, and inventory cleanup.
- A successful node mutation is incomplete until the parent Run has an exact post-commit
  reconciliation wake.
- The native-tool journey found transaction-boundary gaps that descriptor and adapter parity tests
  alone could not reveal.

## Final Status

- **Exit gate:** `make gate` passed after the final Task 07 repository mutation; the content-keyed
  fingerprint and evidence log are recorded by `make gate-status`.
- **Issues by user impact:** 3 Blocks-Completion found, fixed, and same-persona verified; 0 open.
- **Coverage:** 2 of 2 planned charters walked; 3 of 3 affected scenarios are `pass`.
- **Verdict:** Pass after remediation.
