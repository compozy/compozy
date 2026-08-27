---
id: RT-session-calls-inspector-panel
area: RT
title: The session inspector lists calls in both directions
persona: Ada
journey: J-supervise-delegation-trees
expected: The Calls tab lists what a session asked for and what it was asked for, distinguished by arrow rather than colour, each with its own daemon count. A pruned counterpart keeps its identity and its state while the jump link goes absent, and the pager states how many of the real total are loaded.
entry_points: web /agents/reviewer/sessions/ses_01JBD8G2MZTX inspector Calls tab; HTTP and UDS GET /api/workspaces/{workspace_id}/calls?caller=ses_01JBD8G2MZTX&limit=25; HTTP and UDS GET /api/workspaces/{workspace_id}/calls?child_session_id=ses_01JBD8G2MZTX&limit=25
qa_status: pass
bug_ids: 
fix_status: 
retest_status: 
fix_commits: 
evidence: /Users/pedronauck/dev/qa-labs/compozy-agent-comms-20260826-20260826-065104-728050-lab/qa-artifacts/qa/scenario-walk-matrix.md
last_report: docs/qa/reports/2026-08-26-agent-comms.md
overlaps: RT-delegation-activity-tree; RT-in-context-call-messages; RT-agent-call-follow-up
---

Added by task_06. The walk must prove the count on screen equals the daemon count for the same filter while fewer rows are loaded.

Message pages carry no total, so confirm no message surface renders one and that bounded loading
wording stands in for a count rather than a page length pretending to be one. Direction must be
readable without colour — the arrow carries it — and a pruned counterpart must degrade to an absent
link rather than one that 404s.
