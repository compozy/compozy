---
id: RT-agent-roster-call-compose
area: RT
title: The roster describes agents and can ask one for work
persona: Ada
journey: J-build-a-subagent-roster
expected: The catalog shows each definition's description, its scope, and a Shadowed marker on inactive name collisions; a definition with no description renders the gap rather than inventing one. Agent detail offers a Call compose whose invalid contract fails inline with call_expect_invalid, and whose accepted call links to the new record.
entry_points: web /agents and /agents/reviewer; HTTP and UDS GET /api/agents?workspace=ws_main; HTTP and UDS POST /api/workspaces/ws_main/calls with {"target":{"agent":"reviewer"},"prompt":"Review HEAD~1..HEAD"}; compozy agent list -o json; compozy__agent_list with {"workspace_id":"ws_main"}; the reviewer Call compose action
qa_status: pass
bug_ids: 
fix_status: 
retest_status: 
fix_commits: 
evidence: /Users/pedronauck/dev/qa-labs/compozy-agent-comms-20260826-20260826-065104-728050-lab/qa-artifacts/qa/scenario-walk-matrix.md
last_report: docs/qa/reports/2026-08-26-agent-comms.md
overlaps: RT-subagent-roster-injection; RT-agent-call-golden-path; SITE-agent-comms-docs-area
---

Added by task_06. The walk must confirm a zero instance count renders nothing at all, and that the compose reports the daemon's own refusal code.

This is the operator's path into a call, so it must agree with the agent's path: the descriptions,
scopes and shadow markers on screen must match what `compozy agent list` and the injected tool
parameter carry. Also walk the roster's own edges — a zero-definitions empty state, a large roster
rendering bounded, and an unknown or expired target from the compose.
