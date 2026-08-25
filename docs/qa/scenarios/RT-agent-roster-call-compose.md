---
id: RT-agent-roster-call-compose
area: RT
title: The roster describes agents and can ask one for work
persona: Ada
journey: J-08-watch-and-maintain
expected: The catalog shows each definition's description, its scope, and a Shadowed marker on inactive name collisions; a definition with no description renders the gap rather than inventing one. Agent detail offers a Call compose whose invalid contract fails inline with call_expect_invalid, and whose accepted call links to the new record.
entry_points: web /agents; web /agents/{name}; POST /api/calls; GET /api/agents
qa_status: untested
bug_ids: 
fix_status: 
retest_status: 
fix_commits: 
evidence: 
last_report: 
overlaps: 
---

Added by task_06. The walk must confirm a zero instance count renders nothing at all, and that the compose reports the daemon's own refusal code.
