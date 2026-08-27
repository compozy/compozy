---
id: RT-delegation-activity-tree
area: RT
title: Activity shows every delegation tree, live
persona: Ada
journey: J-supervise-delegation-trees
expected: /agents/activity groups calls by governed root, indents each row by the daemon's own depth, escalates the worst state onto a folded header, and opens the call record from a row. Per-tree counts come from CallsResponse.total on root_session_id-filtered reads, never from the loaded page.
entry_points: web /agents/activity; HTTP and UDS GET /api/workspaces/{workspace_id}/calls?root_session_id=ses_01JBD7ZZAAAA&limit=200; compozy call list --caller ses_01JBD7ZZAAAA --state queued,running,completed --limit 200
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-agent-comms-20260826-20260826-065104-728050-lab/qa-artifacts/qa/scenario-walk-matrix.md
last_report: docs/qa/reports/2026-08-26-agent-comms.md
overlaps: RT-session-calls-inspector-panel; RT-session-stop-subtree; RT-call-record-terminal-states
---

Added by task_06. The walk must prove a folded tree still shows its worst state, that the header count equals the daemon count for the same filter, and that keyboard traversal reaches every row.

Parked children must read distinct from running and from gone, and there must be no Revive control
anywhere — revival is calling or messaging a parked child, so the affordances are call-again and
message. Liveness here is a poll, not a browser-reachable stream: confirm the stale signal borrowed
from the session catalog stream degrades the view honestly rather than freezing it, and that the
empty state teaches the feature instead of rendering a zero.
