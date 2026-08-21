---
id: RT-session-catalog-server-scope
area: RT
title: Session catalog scope is enforced by the daemon
persona: Lea
journey: J-15
expected: Session list and catalog-stream requests require exactly one of workspace_id or all_workspaces, carry the selected profile or all_profiles scope, and preserve it through initial reads, live events, and Last-Event-ID replay. A workspace-scoped list or stream never includes sessions from another workspace or profile; aggregate scope includes every requested workspace and profile, including sessions without a workspace.
entry_points: compozy session list; GET /api/sessions and GET /api/sessions/catalog-stream over HTTP and UDS; web session catalog; profile=<name>; all_profiles=true; all_workspaces=true
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-profile-stream-isolation; RT-session-attention-catalog
---

story: As an operator, I can choose one project or Global and trust the daemon to enforce that catalog boundary for pages and live updates.

2026-08-21 qa-impact: list and stream scope moved to the shared server handler, including replay. Task 13 owns the live walk.

The profile-stream scenario owns the profile-filtered initial, live, and replay assertions; this row
must still compare CLI, HTTP, and UDS for the workspace/all-workspaces contract and cross-link that
owner rather than silently omitting profile and transport coverage.
