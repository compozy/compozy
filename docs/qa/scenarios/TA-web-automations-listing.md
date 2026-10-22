---
id: TA-web-automations-listing
area: TA
title: One Automations listing shows every job and trigger as a sentence with its last run
persona: Cora
journey: J-24
expected: "`/automations` lists jobs and triggers together, sorted by name (schedules first on ties); the head reads Automations with the total `jobs.page.total + triggers.page.total` and one secondary New automation. Start views All · Scheduled · On events (webhooks count as events) carry per-list counts and `?start=`; search `q` and the Does · Status · Source · Loop filters (plus the shared multi-select Project filter, only under All projects) are server-side and round-trip through the URL; Clear filters resets `q`, every facet and `start`; an unknown `start` normalizes away. Each row is the shared sentence with Off / From config / From package badges only, the time stat (jobs: next run; triggers: last ran; faint — when Off or never ran), the last-run truth (Last run failed in danger; skipped, missed, canceled, running, handed off, completed neutral; nothing when never ran), a non-optimistic On/Off switch (dimmed while pending, previous state + toast on failure), and an overflow with only supported actions (Run now for schedules, Copy link for webhooks, Edit/Delete for dynamic, disabled Edit in config.toml for config). Rows|Cards toggle keeps the same content. The footer reads N automations · M on · next run in X. Load more appears when either list has more. Loading skeletons, the unavailable alert with Open Settings, the both-failed load error with Try again, and the one-list-failed inline alert (that Start view count reads —) are explicit. Breadth follows the destination: a focused project lists only that project's automations (`scope=workspace&workspace_id`, decision 8A), User lists only automations without a project (`scope=global`), and All projects lists every project's plus User's (`all_workspaces=true`, Project filter → `workspace_ids`, User = `@user`) with a project badge (User for no project) on every row and card; the filter resets when the mode changes. An empty unfiltered All projects listing offers every project's suggestions from the aggregate, each naming its project. No surface says Global."
entry_points: web `/automations`; dock Automations launcher
qa_status: untested
bug_ids: Q1-F5 fixed; Q1-F6 fixed; Q1-F8 fixed; Q1-F9 pre-existing on main, out of scope
fix_status: none
retest_status: untested
fix_commits: 99910ca2b
evidence: .compozy/tasks/automations/screens/; .compozy/tasks/automations/reports/q1-qa-walk.md (Round 2)
last_report: .compozy/tasks/automations/reports/q1-qa-walk.md
overlaps: TA-web-automations-first-run; TA-web-automation-detail; TA-automation-last-run-agent; ET-web-route-chrome-topbar; ET-web-page-content-gutter
---

Replaces `ET-web-jobs-triggers-catalog` (Automations spec, `.compozy/tasks/automations/`, task 07; US-001–US-007, US-009). The Jobs and Triggers catalogs merged into one listing; the predecessor's history lives in its dated reports (`docs/qa/reports/2026-10-02-untested.md` and earlier).

Fixture: the board's seven automations (`morning-digest`, `nightly-delivery`, `dependency-review`, `release-checklist`, `rerun-delivery`, `summarize-failures`, `deploy-webhook`) with at least one failed, one skipped (`self_overlap`), one config-sourced and one Off. Visual contract: `docs/design/opendesign/automations/automations-list.html` (list VC-01…06).

Regression obligations carried over from the predecessor and still owned here:

- an offline or 503 catalog keeps cached rows with an explicit waiting state and resumes pagination (BUG-20261004-automation-offline-pagination-silent);
- User scope (formerly Global) survives retained scoped Loop windows and a failed User job (BUG-20261003-background-windows-forget-global);
- concurrent Run now on different rows keeps each disabled only until its own request settles; runtime-off disables every Run now;
- workspace-scoped rows, details and mutations are withheld unless `workspace_id` matches the active project.

Walk includes aggregate mode (owner tag in row meta), a job and a trigger sharing one name (both rows, distinct detail routes), a single-kind inventory (On events count 0), and keyboard operation of the Start views (pressed state) and row switch ("Turn <name> on or off").

All-projects AW4 (2026-10-21): reset to untested. The Location facet (This project / Global) is retired: breadth comes from the destination and, under All projects, the Project filter. New walk: (1) project mode lists only that project's jobs and triggers; (2) User lists only automations without a project; (3) All projects lists both projects plus User with badges, the Project filter narrows to one project + User and shows the filtered-empty state with Clear when nothing matches, and leaving All projects drops the filter; (4) a zero-inventory All projects listing shows suggestions from `/api/automation/suggestions?all_workspaces=true`, each badged, and Create automation lands in that suggestion's own project. PR screenshots: All projects listing with badges; Project filter applied; filtered-empty.
