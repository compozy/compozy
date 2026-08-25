---
id: RT-call-record-terminal-states
area: RT
title: Call detail tells the whole story for each terminal state
persona: Ada
journey: J-08-watch-and-maintain
expected: /agents/calls/{id} renders the ask, contract digest, state timeline, typed result and cost for every one of the nine states; extracted renders as extracted; invalid-result keeps both tries verbatim; completed-without-result says so; a canceled call shows superseded evidence without reopening; a deadline appears only when one was set.
entry_points: web /agents/calls/{call_id}; GET /api/calls/{id}; GET /api/calls/{id}/result; GET /api/calls/{id}/superseded
qa_status: untested
bug_ids: 
fix_status: 
retest_status: 
fix_commits: 
evidence: 
last_report: 
overlaps: 
---

Added by task_06. The walk must confirm each state renders only the controls whose operation exists — cancel in flight, call-again and message once terminal — and that nothing is greyed in place of absent.
