---
id: RT-web-session-all-workspaces
area: RT
title: Browse sessions across every workspace without losing healthy groups
persona: Théo
journey: J-respond-to-agent-attention
expected: The Sessions catalog offers Recent, All, and All workspaces; the widest scope loads every cursor page per live workspace, labels and collapses groups, isolates one workspace's failure, joins and removes workspaces live, persists globally, and opens a foreign session in its owner workspace.
entry_points: web Sessions dock item; web session-window sidebar
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: RT-session-attention-catalog; ET-web-session-cross-workspace-confirm
---

Exercise more than 100 sessions in one workspace, a failing sibling workspace, live workspace
join/removal, collapse, reload, and foreign activation. A failed group must not blank healthy groups,
and actions scoped to the active runtime must never target a foreign row.

QA impact 2026-08-16: Task 03 added the daemon-backed tri-state scope and cursor-complete grouped
catalog. Flag only; Task 08 owns the real-user walk and evidence.
