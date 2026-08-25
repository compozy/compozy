---
id: RT-delegation-attention-signals
area: RT
title: Delegations that need a look reach the bell and the dock
persona: Ada
journey: J-08-watch-and-maintain
expected: An invalid-result or completed-without-result call, and a child blocked on a decision, raise needs-you rows on the Agents tile and in the bell under the existing grammar. A failing fan-out coalesces into one row per tree carrying the real count. Rows clear when their cause resolves; no dismiss, snooze, or clear-all exists.
entry_points: web OS dock and attention bell; GET /api/workspaces/{id}/calls?state=invalid-result,completed-without-result&limit=1
qa_status: untested
bug_ids: 
fix_status: 
retest_status: 
fix_commits: 
evidence: 
last_report: 
overlaps: 
---

Added by task_06. The walk must confirm a stale source contributes zero to the badge while its rows stay clickable, and that a blocked child appears once — as the call row naming its tree, not also as a bare session row.
