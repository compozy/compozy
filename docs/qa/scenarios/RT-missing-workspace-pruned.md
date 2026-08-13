---
id: RT-missing-workspace-pruned
area: RT
title: Prune a workspace whose local folder was removed
persona: Bruno
journey: J-prune-missing-workspace
expected: After a registered local folder is removed, the next reconciliation removes the workspace from Web, CLI, HTTP, and UDS catalogs, removes its stopped session artifacts through the session owner, and recovers old routes without leaving a ghost selection. If Global was on, the chip stays Global (`~`) and the workspace menu shows a deletion notice — it must not fall back to another project folder. If the pruned folder was the remembered project while Global was off, the shell falls back to Global rather than auto-selecting `workspaces[0]`. `$HOME` never appears as a picker row.
entry_points: web workspace picker; CLI compozy workspace list; GET /api/workspaces
qa_status: blocked-verify
bug_ids: BUG-20260713-missing-workspace-persists
fix_status: fixed
retest_status: pass
fix_commits: 8eeb8a38
evidence: /Users/pedronauck/dev/qa-labs/compozy-dev-websocket-recovery-20260803-155044-571985-lab/qa-artifacts/qa/screenshots/ch-prune-recovered-after-refresh.png; /Users/pedronauck/dev/qa-labs/compozy-dev-websocket-recovery-20260803-155044-571985-lab/qa-artifacts/qa/logs/missing-workspace-websocket.log; /Users/pedronauck/dev/qa-labs/compozy-dev-websocket-recovery-20260803-155044-571985-lab/qa-artifacts/qa/logs/workspace-list-cli.json
last_report: docs/qa/reports/2026-08-03-dev-websocket-recovery.md
overlaps: RT-008;RT-009
---

Linear issue Compozy-47 is the named regression target.

2026-07-13: Failed in CH-prune-missing-workspace. Removed registration `ws_73db983811b21119` returned 410 on direct read but remained in Web and `GET /api/workspaces` after fallback selection and a full refresh.

2026-07-13: Passed after remediation. The first Web catalog read after daemon restart removed the ghost, HTTP and UDS CLI lists converged on the same three valid workspaces, the old ID returned 404, and a second daemon restart preserved the deletion.

2026-07-14 final ownership retest: `workspace-prune-final` owned stopped real session `sess-0f2fc3f71bf6b69e` when its folder was removed. The next Browser navigation removed the workspace from the rail, selected `compozy3`, rendered no error, and the UDS list retained only healthy workspaces. The session directory was removed by the staged owner.

QA impact 2026-07-14: workspace unregister now requires the session owner's atomic removal preparer and rejects pending session starts. Reset pending a final-worktree prune replay.

2026-07-14 final-worktree control: the next public catalog reconciliation removed the missing root, preserved the healthy home workspace, and cleared the stale Web selection. Retest promoted to pass.

2026-08-03 targeted recovery retest: removing the active disposable workspace switched the live Web client to the healthy home workspace, survived refresh, removed the old ID from CLI and HTTP catalogs, and returned a terminal WebSocket error frame through Vite without proxy errors.

2026-08-12 qa-impact: missing remembered id while Global is on stays Global with a deletion notice; there is no `workspaces[0]` fallback and `$HOME` is not a UI row. Reset to untested.

2026-08-12 walk: blocked-verify. This implementation cycle captured Storybook visual-contract evidence (`.compozy/tasks/global-workspace-menubar/evidence/visual/menubar-toggle/VC-01`–`VC-04`) and unit/typecheck coverage. An isolated QA lab with a live daemon (`COMPOZY_HOME`, production-parity web) was not started, so a persona walk through public entry points could not meet the qa-execution evidence standard.
