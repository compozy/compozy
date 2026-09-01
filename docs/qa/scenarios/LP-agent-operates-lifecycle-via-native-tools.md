---
id: LP-agent-operates-lifecycle-via-native-tools
area: LP
title: Operate Loop lifecycle through native tools
persona: Ada
journey: J-07
expected: An agent discovers and uses the six Loop lifecycle tools to list workspace-scoped actionable nodes and control runs or nodes with stable structured answers, immediate forced cancellation, deterministic invalid-state reasons, winner provenance, and no retired stop or Kill tool.
entry_points: compozy__loop_cancel; compozy__loop_nodes; compozy__loop_node_pause; compozy__loop_node_resume; compozy__loop_node_cancel; compozy__loop_node_requeue
qa_status: pass
bug_ids: BUG-20260802-initial-wait-fails-run; BUG-20260802-parked-node-cancel-stalls; BUG-20260802-node-kill-leaves-run-live; BUG-20260803-agent-workspace-id-disagrees
fix_status: fixed
retest_status: pass
fix_commits: Task 07 checkpoint
evidence: looprun-cab69e8c2333002e; looprun-86df5fa096c0e4d6; looprun-a7626a6f4ef766f7; looprun-87cc4cc6faa5a112; looprun-55b637796c09ce9b; looprun-48465e2e2366e555
last_report: docs/qa/reports/2026-08-31-issue-500-forced-loop-cancel.md
overlaps: TA-076; LP-forced-cancel-owned-sessions
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

acceptance-walk: From an Ada-scoped session, rediscover the exact six lifecycle tools, invoke every run and node action, and repeat incompatible losers. Compare each structured result with fresh CLI and HTTP state, prove workspace B cannot list or mutate workspace A nodes, and confirm compozy__loop_stop, compozy__loop_kill, and compozy__loop_node_kill remain unknown.

QA impact 2026-08-31: Kill tools were removed. Re-walk the six-tool lifecycle surface and confirm
Cancel is destructive, both Kill IDs are unknown, and fresh reads preserve workspace isolation.
