---
id: RT-missing-workspace-pruned
area: RT
title: Prune a workspace whose local folder was removed
persona: Bruno
journey: J-prune-missing-workspace
expected: After a registered local folder is removed, the next reconciliation removes the workspace from Web, CLI, HTTP, and UDS catalogs, removes its stopped session artifacts through the session owner, and recovers old routes without leaving a ghost selection.
entry_points: web workspace picker; CLI agh workspace list; GET /api/workspaces
qa_status: pass
bug_ids: BUG-20260713-missing-workspace-persists
fix_status: fixed
retest_status: pass
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/agh-automation-features-20260713-20260713-044543-173594-lab/qa-artifacts/qa/screenshots/ch-prune-after-folder-removal.dom.txt; /Users/pedronauck/dev/qa-labs/agh-automation-features-20260713-20260713-044543-173594-lab/qa-artifacts/qa/screenshots/ch-prune-ghost-persists-after-fallback-refresh.dom.txt; /Users/pedronauck/dev/qa-labs/agh-automation-features-20260713-20260713-044543-173594-lab/qa-artifacts/qa/screenshots/rt-missing-workspace-pruned-first-ui.dom.txt
last_report: docs/qa/reports/2026-07-14-consumer-saas-growth.md
overlaps: RT-008;RT-009
---

Linear issue AGH-47 is the named regression target.

2026-07-13: Failed in CH-prune-missing-workspace. Removed registration `ws_73db983811b21119` returned 410 on direct read but remained in Web and `GET /api/workspaces` after fallback selection and a full refresh.

2026-07-13: Passed after remediation. The first Web catalog read after daemon restart removed the ghost, HTTP and UDS CLI lists converged on the same three valid workspaces, the old ID returned 404, and a second daemon restart preserved the deletion.

2026-07-14 final ownership retest: `workspace-prune-final` owned stopped real session `sess-0f2fc3f71bf6b69e` when its folder was removed. The next Browser navigation removed the workspace from the rail, selected `agh3`, rendered no error, and the UDS list retained only healthy workspaces. The session directory was removed by the staged owner.

QA impact 2026-07-14: workspace unregister now requires the session owner's atomic removal preparer and rejects pending session starts. Reset pending a final-worktree prune replay.

2026-07-14 final-worktree control: the next public catalog reconciliation removed the missing root, preserved the healthy home workspace, and cleared the stale Web selection. Retest promoted to pass.
