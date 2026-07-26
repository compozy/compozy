# BUG-20260713-parent-task-rollup-missing: A parent stays non-terminal after every child completes

- **Status:** verified
- **Impact (user-side):** Blocks-Completion
- **Severity:** Critical · **Priority:** P0
- **Persona Affected:** Bruno
- **Journey Step:** J-complete-task-tree, complete the final child and continue the parent workflow
- **Scenarios:** TA-parent-rollup-completion; LP-task-rollup-wakes-loop
- **Found:** 2026-07-13 · **Report:** docs/qa/reports/2026-07-13-automation-features.md
- **Origin:** Linear AGH-71 live Cursor replay

## Summary

All three children of one parent completed through the real UI and Cursor/Grok task-role path, but the parent remained `Ready` with an old `Needs Attention` run after refresh. An earlier child correctly left the parent non-terminal; completing the final child produced no parent completion. The user cannot finish the task tree or trigger downstream work from the parent settlement.

## Reproduction

1. Open parent Task `task-4b4a98ccf636c99b` in charter `CH-task-tree-loop-rollup` / Feature Tour.
2. Recover child `task-f6638f9897b1b0f8`; let its one real Cursor/Grok task-role session claim and complete its continuation.
3. Assign and recover child `task-a090a4e5ba779d61`; let its one real task-role session complete its continuation.
4. Refresh the parent detail and open the Children tab.

**Expected:** The first child completion leaves the parent non-terminal. The final child completion settles the parent exactly once, exposes that state after refresh through every structured surface, and emits at most one downstream wake/event.
**Actual:** All three children render Completed, but the parent remains Ready with Needs Attention on `run-27fa29c0b0feca21`.

## Evidence

- `/Users/pedronauck/dev/qa-labs/agh-automation-features-20260713-20260713-044543-173594-lab/qa-artifacts/qa/screenshots/agh71-child1-completed.dom.txt`
- `/Users/pedronauck/dev/qa-labs/agh-automation-features-20260713-20260713-044543-173594-lab/qa-artifacts/qa/screenshots/agh71-parent-after-first-child.dom.txt`
- `/Users/pedronauck/dev/qa-labs/agh-automation-features-20260713-20260713-044543-173594-lab/qa-artifacts/qa/screenshots/agh71-all-children-completed-parent-stuck.dom.txt`
- Parent `task-4b4a98ccf636c99b`; child continuations `run-e768865c2ff066dd` and `run-9b8470afc9f2c190`.

## Fix

- **Root cause:** Child completion entry points settled the child/run independently and did not share one transactional hierarchy-settlement owner. The final child therefore had no atomic path that re-read sibling terminality, settled the parent once, appended its event, and emitted the dependent coordinator/Loop wake after commit.
- **Fix commit:** pending final whole-diff commit.
- **Regression test:** The canonical Task manager integration suite owns a fresh three-child hierarchy across all Goal-independent completion paths, including a recovered retry. It proves an earlier child leaves the parent non-terminal, the final child settles the parent exactly once, replay/concurrency is idempotent, and event/wake publication does not duplicate. Historical pre-fix parents are intentionally not backfilled in this greenfield contract, so live acceptance must create a fresh tree.

## Verification

- A fresh, uncontaminated tree used parent `task-a2b46ce593b5e75b` with unavailable exact owner `sess-agh71-unavailable-parent`, so the parent run naturally reached `task.run_starved` and `task.run_needs_attention` without ever binding a session.
- Child A `task-aeae6a3825340585` completed once through Cursor/Grok session `sess-0bb0f23ac1414396`; the parent remained nonterminal.
- Child B `task-1f83323b5632a917` completed once through session `sess-64f9badf5a65dd2f`. The final child transaction emitted parent `task.run.completed` seq 300 and `task.status_changed` seq 301, and parent run `run-b0985a94beb209b9` became Completed with `No bound session`.
- A real reload plus Children tab showed the parent and both children Completed, with exactly one parent run and one run per child. Evidence: `agh71-faithful-parent-run.dom.txt`, `agh71-faithful-parent-children.dom.txt`, and `agh71-faithful-child-b-one-run.dom.txt` under the active post-onboarding-fix lab.
- A matching Loop wake was not installed for this fresh tree; that separate integration boundary remains pending in `LP-task-rollup-wakes-loop` and does not weaken the AGH-71 parent-settlement verdict.
