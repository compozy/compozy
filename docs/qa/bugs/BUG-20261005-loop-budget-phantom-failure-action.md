# BUG-20261005-loop-budget-phantom-failure-action: An exhausted Loop sends the operator looking for a failed step that does not exist

- **Status:** open
- **Impact (user-side):** Friction
- **Severity:** Low · **Priority:** P3
- **Persona Affected:** Bruno
- **Journey Step:** J-01, inspect a non-successful terminal run
- **Scenarios:** LP-003
- **Found:** 2026-10-05 · **Report:** docs/qa/reports/2026-10-02-untested.md

## Summary

After a Loop exhausts its wall budget between successful steps, its briefing offers
"Open the failed step". Bruno follows it into Inspect, but there is no failed step to find.
The remaining step is pending because the budget fenced its dispatch.

## Reproduction

- **Charter:** CH-012 · **Tour:** Feature Tour
- **Environment:** desktop, 1512x862 at DPR 2, fast Wi-Fi, en-US

1. Run implement-tasks with a 20-second wall budget and a real task that settles after that limit.
2. Reload the Exhausted run detail after the worker finishes.
3. Follow "Open the failed step" and inspect the graph.

**Expected:** Failed-step navigation requires an actual failed node or gate. Generic Inspect
remains available for examining exhaustion and other run-level outcomes.
**Actual:** The button opens Inspect even though every executed node succeeded and the next
delivery node remains pending. No failed node exists.

## Evidence

Run looprun-c600576569285100 is Exhausted/budget, with 87,043 tokens and 2m55s against
120K/20s. The independent UDS read confirms no failed generation output; per_task_done is
pending without a task_run_id. The fresh Web view and subsequent Inspect graph agree.

Receipts under docs/qa/evidence/2026-10-02-untested/:
loops-budget-action-bruno-probe.json and loops-budget-action-bruno-independent.json;
loops-budget-action-bruno-before.png and loops-budget-action-bruno-after.png.
The exact loops-budget-action-bruno recording is closed with five frames. The screenshot
captures the button and unchanged outcome; the AX read records the opened graph's actual nodes.
This is a navigation/attribution finding, not a claim that budget enforcement failed.

## Fix

Root cause: briefingAction uses tone=failed alone to offer failed-step navigation. The same
tone represents budget exhaustion with an empty blocker list. The action must require a
concrete failure reference; the existing generic Inspect control remains usable.

The briefing projection now requires a failure blocker with a node or gate reference before
offering the action. The existing LoopRunBriefing suite owns availability and navigation:
exhausted/stalled outcomes without that reference omit the action, while concrete node/gate
failures retain Inspect navigation. The focused red run fails the two missing-target cases;
the repaired suite passes all 172 cases. No wire, copy, or primitive changes are needed.

Fix commit: pending final delivery gate.

## Verification

Bruno replays both outcomes on the daemon-served production bundle index-CHherNRv.js. A fresh
document for looprun-c600576569285100 stays Exhausted, omits the misleading action, and opens
the graph through generic Inspect. Run looprun-b1d0fd712e0182b5 stays Failed and retains the
action; following it exposes the genuinely failed collect node. Independent UDS reads agree
with each outcome. Terminal screenshots were visually inspected. The exact recording
/Users/pedronauck/.config/browser-harness/agent-workspace/recordings/loops-budget-action-fixed-bruno
is closed with nine frames; loops-budget-action-fixed-bruno.json records the interaction.

Root Turbo Web typecheck/build passes (4/4 tasks); React Doctor reports 100/100 across seven
changed files. Existing bundle advisories are retained. Final gate and commit remain pending;
this focused repair does not close LP-003's separate stalled-outcome leg.
