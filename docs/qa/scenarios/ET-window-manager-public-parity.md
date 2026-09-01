---
id: ET-window-manager-public-parity
area: ET
title: Manage one window topology through every public surface
persona: Ada
journey: J-administer-window-manager
expected: Native tools, CLI, HTTP, and UDS expose the same semantic desktop, window, navigation, client, preview, layout, and revision contracts; the WebSocket snapshot fence orders later topology events and an optional client fence orders only that ClientView's presentation frames; restart preserves topology and routes; workspace deletion purges them; and no request can observe or mutate another workspace.
entry_points: compozy desktop; compozy window; compozy layout; compozy__window_manager; /api/workspaces/{workspace_id}/window-manager
qa_status: pass
bug_ids:
fix_status:
retest_status: pass
fix_commits: a1baedd3a
evidence: /Users/pedronauck/dev/qa-labs/compozy-window-manager-hardening-20260901-200758-934379-lab/qa-artifacts/qa/test-cases/walk-parity-results.json; /Users/pedronauck/dev/qa-labs/compozy-window-manager-hardening-20260901-200758-934379-lab/qa-artifacts/qa/logs/cli-parity.log; /Users/pedronauck/dev/qa-labs/compozy-window-manager-hardening-20260901-200758-934379-lab/qa-artifacts/qa/logs/cli-parity-desktop.log; /Users/pedronauck/dev/qa-labs/compozy-window-manager-hardening-20260901-200758-934379-lab/qa-artifacts/qa/qa-audit-report.json; /Users/pedronauck/dev/qa-labs/compozy-window-manager-hardening-20260901-200758-934379-lab/qa-artifacts/qa/teardown.json
last_report: docs/qa/reports/2026-09-01-window-manager-hardening.md
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

qa-impact: 2026-09-01 walked P1, P1b, P2 and the zoom walk's CLI check on the structural zoom: `compozy window zoom` without a client returns a lifted window home and lifts it again; `window list` carries `zoomed`; two browser clients converge and only the lifted client's view is repaired.
