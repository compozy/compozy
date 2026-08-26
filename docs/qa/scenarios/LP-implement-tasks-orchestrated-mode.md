---
id: LP-implement-tasks-orchestrated-mode
area: LP
title: Delegate implement-tasks through orchestrated mode
persona: Bruno
journey: J-01
expected: Running implement-tasks with mode=orchestrated uses the bundled orchestrator in one continuous Goal session, gives every task its category-selected worker runtime, proves completed task frontmatter on disk, stops every spawned worker, marks the per-task branch not_taken, and settles done.
entry_points: compozy loop run --name implement-tasks --input slug=<slug> --input mode=orchestrated; compozy loop status; compozy session list --parent <goal-session>; web /loop-runs/:run_id detail
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: LP-003; LP-goal-command-judge; ET-spec-cycle-skill-bundle
---

The conductor delegates only: it may inspect task state and dispatch bounded workers, but it does
not edit production files. The public walk must prove spawn, blocking prompt, on-disk completion,
and stop for each worker, including provider, model, reasoning effort, and speed when supplied.
