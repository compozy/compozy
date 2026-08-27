---
id: RT-call-record-terminal-states
area: RT
title: Call detail tells the whole story for every one of the nine call states
persona: Ada
journey: J-supervise-delegation-trees
expected: /agents/calls/{id} renders the ask, contract digest, state timeline, typed result and cost for every one of the nine states; extracted renders as extracted; invalid-result keeps both tries verbatim; completed-without-result says so; a canceled call shows superseded evidence without reopening; a deadline appears only when one was set.
entry_points: web /agents/calls/call_01JBD8G2K7Q9; HTTP and UDS GET /api/workspaces/{workspace_id}/calls/{call_id}; HTTP and UDS GET /api/workspaces/{workspace_id}/calls/{call_id}/result; HTTP and UDS GET /api/workspaces/{workspace_id}/calls/{call_id}/superseded
qa_status: pass
bug_ids: 
fix_status: 
retest_status: 
fix_commits: 
evidence: /Users/pedronauck/dev/qa-labs/compozy-agent-comms-20260826-20260826-065104-728050-lab/qa-artifacts/qa/scenario-walk-matrix.md
last_report: docs/qa/reports/2026-08-26-agent-comms.md
overlaps: RT-call-return-contract-repair; RT-agent-call-deadline-timeout; RT-delegation-activity-tree
---

Added by task_06. The walk must confirm each state renders only the controls whose operation exists — cancel in flight, call-again and message once terminal — and that nothing is greyed in place of absent.

Every control here maps one-to-one to a `_dx.md` operation, so the negative check is as important as
the positive: no control may exist that the runtime cannot perform. Compare the rendered verdict
against the record's own — `returned`, `extracted` and `repaired` must stay distinguishable — and
confirm the idle-TTL line reads as suspended while running and counting while parked.
