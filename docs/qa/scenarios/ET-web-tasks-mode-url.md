---
id: ET-web-tasks-mode-url
area: ET
title: Tasks mode navigation via URL search param
persona: Bruno
journey: J-24
expected: Tasks List/Kanban/Dashboard/Inbox modes are RouteNav links driven by `?mode=` (not local-only pills); the active mode has `aria-current="page"`; refreshing preserves the mode; switching modes updates the URL without losing workspace scope.
entry_points: web `/tasks`; `/tasks?mode=kanban|dashboard|inbox`
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-web-route-chrome-topbar
---

Added by Route Chrome alignment (2026-07-17). Mode moved from local state into the URL as RouteNav.

QA impact 2026-07-18: changing modes clears the List search before navigation, so returning from
Kanban, Dashboard, or Inbox cannot silently reuse a hidden query.

QA impact 2026-07-18: direct Inbox and Dashboard navigation no longer waits for the unrelated full
task catalog preload. Verify each mode reaches its own loading or error state while the catalog is
slow or unavailable.

QA impact 2026-07-18: direct Dashboard navigation now preloads and reuses its dashboard summary,
scheduler status, scheduler backlog, and Inbox badge queries while continuing to skip the unrelated
full task catalog.
