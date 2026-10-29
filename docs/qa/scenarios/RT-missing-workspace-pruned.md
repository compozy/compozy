---
id: RT-missing-workspace-pruned
area: RT
title: Retain missing projects without blocking healthy workspaces
persona: Bruno
journey: J-prune-missing-workspace
expected: A missing registered folder remains visible with root_missing and retains its sessions. Healthy projects continue creating worktrees and isolated sessions/subagents. Status and describe return retained state with workspace_root_missing diagnostics; stop and archive remain usable. Aggregate catalogs preserve healthy rows and diagnose unavailable projects.
entry_points: web workspace picker; CLI compozy workspace list; GET /api/workspaces
qa_status: blocked-verify
bug_ids: BUG-20260713-missing-workspace-persists
fix_status: fixed
retest_status: pending
fix_commits: 8eeb8a38
evidence: /Users/pedronauck/dev/qa-labs/compozy-dev-websocket-recovery-20260803-155044-571985-lab/qa-artifacts/qa/screenshots/ch-prune-recovered-after-refresh.png; /Users/pedronauck/dev/qa-labs/compozy-dev-websocket-recovery-20260803-155044-571985-lab/qa-artifacts/qa/logs/missing-workspace-websocket.log; /Users/pedronauck/dev/qa-labs/compozy-dev-websocket-recovery-20260803-155044-571985-lab/qa-artifacts/qa/logs/workspace-list-cli.json
last_report: docs/qa/reports/2026-08-03-dev-websocket-recovery.md
overlaps: RT-008; RT-009
---

## Current regression contract: GitHub #1011

The retention contract supersedes the historical automatic-pruning expectations below.
Use a disposable daemon with a unique home, socket, and port.

1. Register projects A and B; B must be a Git repository. Create an active session in A, then delete only A's disposable directory.
2. List workspaces repeatedly through CLI and HTTP. A remains registered with `root_missing: true`; its session and history remain available.
3. From B, create a worktree through CLI and `compozy__worktree_create`; spawn with `new_worktree`; delegate with `isolation: "worktree"`. Check each resulting checkout and binding belongs to B.
4. Read A's session through CLI status, HTTP detail/status/inspect (including recent wake events), and native status/describe. Expect retained state plus `workspace_root_missing`, not an error caused by loading its agent files.
5. Stop A's session, wait for stopped state, then archive it. Verify the same session ID and history remain. New execution in A still refuses its missing root.
6. Read attention, all-project sessions/worktrees/agents/loops, and automation jobs. Healthy rows remain available; normal and attention session lists diagnose A on each retained row; agents diagnose the skipped A catalog and report `sessions_available: false`. Configured automation definitions retain registered identity when A disappears; healthy definitions still synchronize. Repeat configured automation lookup and retained-session archive using A's registered absolute path.

## Historical evidence

Linear issue Compozy-47 covered the former pruning behavior.

2026-07-13: Failed in CH-prune-missing-workspace. Removed registration `ws_73db983811b21119` returned 410 on direct read but remained in Web and `GET /api/workspaces` after fallback selection and a full refresh.

2026-07-13: Passed after remediation. The first Web catalog read after daemon restart removed the ghost, HTTP and UDS CLI lists converged on the same three valid workspaces, the old ID returned 404, and a second daemon restart preserved the deletion.

2026-07-14 final ownership retest: `workspace-prune-final` owned stopped real session `sess-0f2fc3f71bf6b69e` when its folder was removed. The next Browser navigation removed the workspace from the rail, selected `compozy3`, rendered no error, and the UDS list retained only healthy workspaces. The session directory was removed by the staged owner.

QA impact 2026-07-14: workspace unregister now requires the session owner's atomic removal preparer and rejects pending session starts. Reset pending a final-worktree prune replay.

2026-07-14 final-worktree control: the next public catalog reconciliation removed the missing root, preserved the healthy home workspace, and cleared the stale Web selection. Retest promoted to pass.

2026-08-03 targeted recovery retest: removing the active disposable workspace switched the live Web client to the healthy home workspace, survived refresh, removed the old ID from CLI and HTTP catalogs, and returned a terminal WebSocket error frame through Vite without proxy errors.

2026-08-20 qa-impact: missing remembered id while Global is on stays Global with no deletion notice in the workspace menu; there is no `workspaces[0]` fallback and `$HOME` is not a UI row. Walk this cycle: blocked-verify — active-workspace and workspace-menu unit suites assert Global stays on, no fallback, and the menu has no note/alert; an isolated QA lab with a live daemon was not started.

2026-08-12 qa-impact: missing remembered id while Global is on stays Global with a deletion notice; there is no `workspaces[0]` fallback and `$HOME` is not a UI row. Reset to untested.

2026-08-12 walk: blocked-verify. This implementation cycle captured Storybook visual-contract evidence (`.compozy/tasks/global-workspace-menubar/evidence/visual/menubar-toggle/VC-01`–`VC-04`) and unit/typecheck coverage. An isolated QA lab with a live daemon (`COMPOZY_HOME`, production-parity web) was not started, so a persona walk through public entry points could not meet the qa-execution evidence standard.
