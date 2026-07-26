---
id: LP-toggle-loop-goal
area: LP
title: Run the same custom Loop with and without a goal
persona: Bruno
journey: J-06
expected: A custom Loop can publish and run with a concrete goal/definition-of-done, then publish and run without an optional goal; each fresh run renders only the contract actually saved.
entry_points: web loop editor; web loop detail; web loop run modal
qa_status: untested
bug_ids: BUG-20260713-loop-contract-goal-not-editable
fix_status: fixed
retest_status: pass
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/agh-automation-features-20260713-20260713-044543-173594-lab/qa-artifacts/qa/screenshots/ch-007-loop-fork-run-stopped.dom.txt; /Users/pedronauck/dev/qa-labs/agh-automation-features-20260713-20260713-044543-173594-lab/qa-artifacts/qa/screenshots/ch-loop-goalless-published-v1.dom.txt; /Users/pedronauck/dev/qa-labs/agh-automation-features-20260713-20260713-044543-173594-lab/qa-artifacts/qa/screenshots/ch-loop-goalless-v1-real-run.dom.txt
last_report: docs/qa/reports/2026-07-13-automation-features.md
overlaps: LP-019;LP-024
---

The goal-bearing and goal-less versions must remain distinct after refresh; no
stale Goal UI or evaluator state may leak from the earlier version.

2026-07-13: failed in CH-loop-goal-delete. The workspace Loop editor can add/configure a graph Goal action, but no UI edits `contract.goal` or `definition_of_done`; DSL is explicitly read-only and Configure says these fields require the builder. The same Loop therefore cannot be published and run with then without its optional contract goal through the UI.

2026-07-13: passed same-persona retest. The earlier goal-bearing workspace version started as `looprun-7e6dbcacdf292853`. After the fix, the Contract rail cleared the optional goal and published a goal-less version whose fresh Run projection omitted the goal. A strict second replay published goal-less `reviews-watch` v1 and started real run `looprun-c0e322b615e43c12`; its detail rendered only the saved definition of done, with no stale goal state.

2026-07-23: qa_status reset to untested — the loop run detail page was redesigned per LOOP-RUN-REDESIGN-SPEC.md (plain-language story timeline, Needs You card, terminal outcome cards, group progress bar, Usage/About rail, Inspect drawer); the pass verdict predates that surface.
