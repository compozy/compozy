---
id: LP-web-run-nested-child-runs
area: LP
title: Track nested child runs from the parent run
persona: Bruno
journey: J-await-child-loop
expected: A step parked on its child run (`awaiting child`) wears the delegated glyph — an info ring around a dot — that never reads as `pending`'s dashed ring, in Progress, the fan-out band and the Inspect graph alike. On the graph, a plain step that started a child carries a quiet line under its state naming the step the child is on and how long it has sat there (or the child's status once settled). Opening that step's panel shows its child open; Progress keeps a closed-by-default "N child runs" disclosure under every step that started loop runs. Each child row names the child loop (linked to its run), its item slot when one fanned node started several, and the inputs that set it apart from its siblings; its run status; the step it is on with the park reason in plain words and the count of other live steps; the time it has been on that step, which keeps counting while the step is parked (a durable wait is timed from its wait cell); the served step count from the child's briefing (never "0 of 0"); and the run's elapsed time. A child that started loops of its own opens them a level down, up to three levels. Past eight children the rest wait behind "Show N more".
entry_points: web /loop-runs/:id Progress; web /loop-runs/:id Inspect Graph and node panel; GET /loop-runs/:id/nodes; GET /loop-runs/:id/briefing; GET /loop-runs/:id
qa_status: pass
bug_ids: 705
fix_status: fixed
retest_status: pass
fix_commits:
evidence: https://github.com/compozy/compozy/pull/708
last_report: docs/qa/reports/2026-10-07-issue-705-nested-child-runs.md
overlaps: LP-web-run-operator-register; LP-run-loop-await-child-ordering; LP-fanout-progress-naming
---

story: As an operator running loops within loops I see, on the parent run and from the graph I work in, which step each child — and each child's child — is on, whether it is moving, waiting or stuck, for how long, and which inputs it was started with, without opening each child.

steps:
1. Author three agent-free Loops with `concurrency: allow` where needed: `fix-one-batch` (input `batch`) parks on a durable `wait` (`for: 40m`); `run-one-wave` fans three batches into `fix-one-batch` via `run-loop` `mode: await` with `inputs: { batch: "{{ .item }}" }`; `nightly-waves` runs `run-one-wave` from a single `wave` step with `mode: await`.
2. Start `nightly-waves` and confirm through `GET /loop-runs/<id>/nodes` that `wave` and the three `fix_batch` branches read `awaiting_child` with a `child_loop_run_id`.
3. Pause one grandchild's wait step with `compozy loop node pause`.
4. Open Inspect → Graph. Confirm `wave` shows the delegated glyph, distinct from the dashed `pending` ring of `wave_ok`, and a child line naming `fix batch` with a time that grows.
5. Open the `wave` panel. Confirm the child row (`run-one-wave`, `wave: 2`, `Running`, `At fix batch — waiting on a child run`, time on step, `1 of 7 steps`) and, opened a level down, three grandchildren labelled by item slot with `batch: api|billing|docs`, the paused one reading `— paused`, each with its time on step.
6. In Progress, confirm `wave` keeps a collapsed "1 child run" that opens to the same rows.
