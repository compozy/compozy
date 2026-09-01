---
id: ET-window-manager-public-parity
area: ET
title: Manage one window topology through every public surface
persona: Ada
journey: J-administer-window-manager
expected: Native tools, CLI, HTTP, and UDS expose the same semantic desktop, window, navigation, client, preview, layout, and revision contracts; the WebSocket snapshot fence orders later topology events and an optional client fence orders only that ClientView's presentation frames; restart preserves topology and routes; workspace deletion purges them; and no request can observe or mutate another workspace.
entry_points: compozy desktop; compozy window; compozy layout; compozy__window_manager; /api/workspaces/{workspace_id}/window-manager
qa_status: untested
bug_ids:
fix_status:
retest_status: pass
fix_commits:
evidence: docs/qa/evidence/2026-08-01-window-tabs/agent-01-cli-route-parity.png
last_report: docs/qa/reports/2026-08-01-window-tabs.md
overlaps: ET-window-manager-multi-client; ET-window-manager-layout-recovery; ET-web-desktop-shell-lifecycle
---

story: As an agent, I can inspect and change the same authoritative window topology without depending on the Web UI.

scope: Exercise structured read and mutation parity, exact expected revisions, stale conflicts, previews with zero writes, stream fencing, restart durability, workspace isolation, and reversible purge through a real daemon.

qa-impact: 2026-07-22 introduced the complete semantic window-manager surface and deleted raw key-value desktop control. Flag only; the next QA cycle owns live retesting.

qa-impact: 2026-07-31 added tab commands, payload extensions, list fields, deterministic errors, and
v3 persistence across CLI, HTTP, UDS, native tools, and streams. Reset for cross-surface parity.

qa-impact: 2026-09-01 `compozy desktop create` lost `--purpose` and `--focus-owner`; `compozy window
zoom` and `compozy__window_zoom` no longer require a client; `compozy window list` and the wire window
carry `zoomed`; the stream contract gained a `heartbeat` frame the CLI stream reader tolerates. Reset for
a CLI/HTTP/UDS/tool parity re-walk.
