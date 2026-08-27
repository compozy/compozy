---
id: RT-delegation-attention-signals
area: RT
title: Delegations that need a look reach the bell and the dock
persona: Ada
journey: J-supervise-delegation-trees
expected: An invalid-result or completed-without-result call, and a child blocked on a decision, raise needs-you rows on the Agents tile and in the bell under the existing grammar. A failing fan-out coalesces into one row per tree carrying the real count. Rows clear when their cause resolves; no dismiss, snooze, or clear-all exists.
entry_points: web OS dock and attention bell from /agents/activity; HTTP and UDS GET /api/workspaces/{workspace_id}/calls?state=invalid-result,completed-without-result&attention=true&limit=1
qa_status: pass
bug_ids: 
fix_status: 
retest_status: 
fix_commits: 
evidence: /Users/pedronauck/dev/qa-labs/compozy-agent-comms-20260826-20260826-065104-728050-lab/qa-artifacts/qa/scenario-walk-matrix.md
last_report: docs/qa/reports/2026-08-26-agent-comms.md
overlaps: RT-delegation-activity-tree; RT-agent-call-batch; RT-call-record-terminal-states
---

Added by task_06. The walk must confirm a stale source contributes zero to the badge while its rows stay clickable, and that a blocked child appears once — as the call row naming its tree, not also as a bare session row.

The dock badge on the existing Agents descriptor is part of this row: needs-you call states must
light it the same way sessions and tasks do, with no new glyph and no new catalog entry. Confirm
there is no budget-exhausted row to find — completions are never admission-denied, so no such state
exists to render — and that finished rows follow the shell's existing finished-section grammar
rather than inventing one.
