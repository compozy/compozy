---
id: LP-web-run-attention-quarantine-routing
area: LP
title: Run attention panel collapses dependency parks and routes to the producer's quarantine entry
persona: Dora
journey: J-05
expected: On a run where one node (e.g. execute_task) is quarantined and several downstream nodes are flagged `dependency_quarantined`, the Needs attention panel renders ONE warning row per quarantined producer — "execute task is quarantined" with "collect, review, verify and approve are parked behind it until it is requeued." — instead of near-identical boilerplate cards per consumer. The row carries a single "Open quarantine entry" button that opens the PRODUCER's repair record (episodes, attempts, requeue), never an empty sheet. Non-dependency flags (silence, expired_wait) keep their own row with the daemon's reason. If the sheet is ever opened for a node without a record it explains that the node is not quarantined and points at the Needs attention panel instead of the dead-end "The daemon has not recorded a repair record for this node." The run timeline gains a `node_attention_flagged` event when consumers park behind a quarantine (waits do not emit it). Requeueing the producer clears the consumer flags and the panel.
entry_points: web loop run detail (looprun-*) -> Needs attention
qa_status: blocked-decision
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: TA-web-task-list-loop-subtask-nesting; LP-web-run-session-one-click
---

story: As the person a run parks for, the panel must tell me the one thing that is actually wrong (the producer) and hand me its repair record in one click — four copies of the same paragraph with buttons that open nothing is the failure this scenario guards against.

The daemon now records `attention_producer_node_id` on dependency-flagged node controls (migration 00055) and the requeue fence clears consumers by that structured field instead of string-matching the reason text. The panel groups consumers per producer and the button routes by producer id into `LoopQuarantineSheet`.

blocked-decision: walking this requires a fresh implement-tasks dogfood run that reaches quarantine (spawns real ACP agent sessions on the operator's account); pending the operator starting one.

src: web/src/systems/loops/components/run-page/loop-run-parked-panels.tsx; web/src/systems/loops/components/run-page/loop-quarantine-sheet.tsx; internal/loop/coordinator_quarantine.go; internal/store/globaldb/global_db_loop_node_requeue.go

inventory: Needs QA
