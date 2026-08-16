---
id: RT-session-attention-catalog
area: RT
title: Discover sessions that need attention across workspaces
persona: Ada
journey: J-15
expected: CLI, HTTP, and UDS return the same workspace-scoped attention catalog, exact badge filters, stable attention ordering, and 100-plus-session summary totals; operator-only cross-workspace and summary reads succeed while agent identity is confined to same-workspace interaction discovery.
entry_points: compozy session list --attention/--badge/--all-workspaces/--summary; compozy session interactions <session-id>; GET /api/sessions/attention-summary; GET /api/workspaces/{workspace_id}/sessions/{session_id}/interactions
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: RT-021; RT-session-clarification-roundtrip
---

Create sessions with permission, clarification, done, failed, and idle badges in at least two
workspaces. Compare counted pages, exact filters, stable cursors, summaries, structured status, and
sanitized interaction discovery across CLI, HTTP, and UDS. Confirm cross-workspace data never enters
a workspace-scoped page or cache, and agent identity receives the documented scope denial outside
same-workspace interaction discovery.

QA impact 2026-08-16: Task 01 added the canonical attention catalog, cross-workspace operator view,
exact summary, status badge, and interaction discovery surfaces. Flag only; task_08 owns execution.
