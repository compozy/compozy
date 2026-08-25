---
id: RT-session-calls-inspector-panel
area: RT
title: The session inspector lists calls in both directions
persona: Ada
journey: J-08-watch-and-maintain
expected: The Calls tab lists what a session asked for and what it was asked for, distinguished by arrow rather than colour, each with its own daemon count. A pruned counterpart keeps its identity and its state while the jump link goes absent, and the pager states how many of the real total are loaded.
entry_points: web session inspector Calls tab; GET /api/workspaces/{id}/calls?caller=; GET /api/workspaces/{id}/calls?child_session_id=
qa_status: untested
bug_ids: 
fix_status: 
retest_status: 
fix_commits: 
evidence: 
last_report: 
overlaps: 
---

Added by task_06. The walk must prove the count on screen equals the daemon count for the same filter while fewer rows are loaded.
