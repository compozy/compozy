---
id: LP-web-run-nested-child-runs
area: LP
title: Track nested child runs from the parent run's Progress
persona: Bruno
journey: J-await-child-loop
expected: On a parent run whose step started loop runs, the step's `awaiting child` chip wears the delegated glyph (an info ring around a dot) that never reads as `pending`'s dashed ring, in Progress and in the Inspect graph alike. Under that step a closed-by-default "N child runs" disclosure opens one row per child, in branch order, naming the child loop (linked to its run page) and, for a fanned node, its item slot. Each row shows the child's run status, the step it is on with the park reason in plain words ("At hold for release — waiting on something", "— paused", "— waiting for your decision") and the count of other live steps, the served step count from the child's briefing ("1 of 2 steps", never "0 of 0"), and an elapsed clock that ticks while the child runs. A settled child names no step. Opening the disclosure reads only the children shown; past eight children the rest wait behind "Show N more".
entry_points: web /loop-runs/:id Progress; web /loop-runs/:id Inspect Graph; GET /loop-runs/:id/nodes; GET /loop-runs/:id/briefing; GET /loop-runs/:id
qa_status: pass
bug_ids: 705
fix_status: fixed
retest_status: pass
fix_commits:
evidence: https://github.com/compozy/compozy/issues/705
last_report: docs/qa/reports/2026-10-07-issue-705-nested-child-runs.md
overlaps: LP-web-run-operator-register; LP-run-loop-await-child-ordering; LP-fanout-progress-naming
---

story: As an operator running loops within loops I see, on the parent run, which step each child run is on, whether it is waiting, paused or finished, and for how long — without opening each child.

steps:
1. Author an agent-free child Loop with `concurrency: allow` whose middle step is a durable `wait` (`for: 40m`), and a parent Loop that fans three items into a `run-loop` node with `mode: await`.
2. Start the parent and confirm through `GET /loop-runs/<parent>/nodes` that every branch reads `awaiting_child` with a `child_loop_run_id`.
3. Pause one child's wait step with `compozy loop node pause` so the children differ.
4. Open the parent run page. Confirm each `awaiting child` chip shows the delegated glyph, distinct from the dashed `pending` ring on the downstream step, and the same in the Inspect graph.
5. Confirm "3 child runs" is collapsed by default and reads no child until opened.
6. Open it. Confirm three rows labelled by item slot, each with the child loop name linked to its run, `Running` status, the current step with its park reason (`waiting on something` twice, `paused` once), `1 of 2 steps`, and a ticking elapsed clock.
