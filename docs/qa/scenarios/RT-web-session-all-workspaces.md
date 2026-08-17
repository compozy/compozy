---
id: RT-web-session-all-workspaces
area: RT
title: Browse sessions across every workspace without losing healthy groups
persona: Théo
journey: J-respond-to-agent-attention
expected: The Sessions catalog offers Recent, All, and All workspaces; the widest scope loads every cursor page per live workspace, labels and collapses groups, isolates one workspace's failure, joins and removes workspaces live, persists globally, and opens a foreign session in its owner workspace.
entry_points: web Sessions dock item; web session-window sidebar
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: docs/qa/reports/2026-08-16-herdr-parity.md; /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260816-141901-835450-lab/qa-artifacts/qa/screenshots/herdr-cross-workspace-needs-you-fixed.png; /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260816-141901-835450-lab/qa-artifacts/qa/screenshots/herdr-attention-all-quiet-cleared.png; .compozy/tasks/herdr-parity/evidence/visual/task_03
last_report: docs/qa/reports/2026-08-16-herdr-parity.md
overlaps: RT-session-attention-catalog; ET-web-session-cross-workspace-confirm
---

Exercise more than 100 sessions in one workspace, a failing sibling workspace, live workspace
join/removal, collapse, reload, and foreign activation. A failed group must not blank healthy groups,
and actions scoped to the active runtime must never target a foreign row.

QA impact 2026-08-16: Task 03 added the daemon-backed tri-state scope and cursor-complete grouped
catalog. Flag only; Task 08 owns the real-user walk and evidence.

QA 2026-08-16 Herdr parity: The isolated browser journey, focused attention Playwright lane, and full Web E2E exercised cross-workspace landing, permission resolution, counts, channel suppression, task canary, catalog scope/order, finished presence clearing, and honest quiet/stale states. The lab browser exposed its real notification capability; deterministic granted and denied branches ran in the canonical browser suite.
