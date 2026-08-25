---
id: RT-delegation-activity-tree
area: RT
title: Activity shows every delegation tree, live
persona: Ada
journey: J-08-watch-and-maintain
expected: /agents/activity groups calls by governed root, indents each row by the daemon's own depth, escalates the worst state onto a folded header, and opens the call record from a row. Per-tree counts come from CallsResponse.total on root_session_id-filtered reads, never from the loaded page.
entry_points: web /agents/activity; GET /api/workspaces/{id}/calls?root_session_id=&limit=1; compozy call list
qa_status: untested
bug_ids: 
fix_status: 
retest_status: 
fix_commits: 
evidence: 
last_report: 
overlaps: 
---

Added by task_06. The walk must prove a folded tree still shows its worst state, that the header count equals the daemon count for the same filter, and that keyboard traversal reaches every row.
