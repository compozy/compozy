---
id: RT-session-catalog-server-scope
area: RT
title: Session catalog scope is enforced by the daemon
persona: Lea
journey: J-operate-session-catalog
expected: Session list and catalog-stream requests require exactly one of workspace_id or all_workspaces. A workspace-scoped list, live stream, and Last-Event-ID replay never include sessions from another workspace. The aggregate scope includes every workspace and sessions without a workspace.
entry_points: GET /api/sessions; GET /api/sessions/catalog-stream; web session catalog
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: RT-session-attention-catalog
---

story: As an operator, I can choose one project or Global and trust the daemon to enforce that catalog boundary for pages and live updates.

2026-08-21 qa-impact: list and stream scope moved to the shared server handler, including replay. Task 13 owns the live walk.
