---
id: ET-window-manager-public-parity
area: ET
title: Manage one window topology through every public surface
persona: Ada
journey: J-administer-window-manager
expected: Native tools, CLI, HTTP, and UDS expose the same semantic desktop, window, navigation, client, preview, layout, and revision contracts; the WebSocket snapshot fence orders later topology events and an optional client fence orders only that ClientView's presentation frames; restart preserves topology and routes; workspace deletion purges them; and no request can observe or mutate another workspace.
entry_points: compozy desktop; compozy window; compozy layout; compozy__window_manager; /api/workspaces/{workspace_id}/window-manager
qa_status: blocked-verify
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-qa-et-current-source-20260730-061655-910372-lab/qa-artifacts/qa
last_report: docs/qa/reports/2026-07-28-untested-full.md
overlaps: ET-window-manager-multi-client; ET-window-manager-layout-recovery; ET-web-desktop-shell-lifecycle
---

story: As an agent, I can inspect and change the same authoritative window topology without depending on the Web UI.

scope: Exercise structured read and mutation parity, exact expected revisions, stale conflicts, previews with zero writes, stream fencing, restart durability, workspace isolation, and reversible purge through a real daemon.

qa-impact: 2026-07-22 introduced the complete semantic window-manager surface and deleted raw key-value desktop control. Flag only; the next QA cycle owns live retesting.
