---
id: LP-agent-operates-lifecycle-via-native-tools
area: LP
title: Operate Loop lifecycle through native tools
persona: Ada
journey: J-07
expected: An agent discovers and uses the eight Loop lifecycle tools to list workspace-scoped actionable nodes and control runs or nodes with stable structured answers, deterministic invalid-state reasons, winner provenance, and no retired stop tool.
entry_points: compozy__loop_cancel; compozy__loop_kill; compozy__loop_nodes; compozy__loop_node_pause; compozy__loop_node_resume; compozy__loop_node_cancel; compozy__loop_node_kill; compozy__loop_node_requeue
qa_status: pass
bug_ids: BUG-20260802-initial-wait-fails-run; BUG-20260802-parked-node-cancel-stalls; BUG-20260802-node-kill-leaves-run-live
fix_status: fixed
retest_status: pass
fix_commits: Task 07 checkpoint
evidence: looprun-cab69e8c2333002e; looprun-86df5fa096c0e4d6; looprun-a7626a6f4ef766f7; looprun-87cc4cc6faa5a112; looprun-55b637796c09ce9b; looprun-48465e2e2366e555
last_report: docs/qa/reports/2026-08-02-loop-node-lifecycle-task07.md
overlaps: TA-076
---

QA impact 2026-08-02: Task 07 adds the agent-facing lifecycle surface and removes
`compozy__loop_stop`. The walk must independently fresh-read the resulting state and prove a
workspace-B inventory cannot leak into workspace A.

QA execution 2026-08-02: all eight lifecycle IDs were discovered and invoked. Pause/resume,
node cancel/kill, run cancel/kill, inventory, and a repeated non-quarantined requeue loser returned
stable structured results. The owner workspace listed one live wait while the neighboring workspace
listed zero and could not mutate the Run. `compozy__loop_stop` was unknown. Three candidate defects
were fixed and re-walked in the same isolated lab; the repaired runs and report above are the durable
evidence.
