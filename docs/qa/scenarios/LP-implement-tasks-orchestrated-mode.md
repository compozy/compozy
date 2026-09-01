---
id: LP-implement-tasks-orchestrated-mode
area: LP
title: Delegate implement-tasks through orchestrated mode
persona: Bruno
journey: J-01
expected: Running implement-tasks with mode=orchestrated and implementer=custom_implementer uses the bundled orchestrator in one continuous Goal session, starts every worker with that exact Agent and its Agent-local sentinel skill, gives every task its category-selected runtime, proves completed task frontmatter on disk, stops every spawned worker, marks the per-task branch not_taken, and settles done. Omitting implementer selects code_implementer.
entry_points: compozy loop run --name implement-tasks --input slug=<slug> --input mode=orchestrated --input implementer=custom_implementer; compozy loop status; compozy session list --parent <goal-session> --agent custom_implementer; web /loop-runs/:run_id detail
qa_status: blocked-verify
bug_ids: BUG-20260826-optional-runtime-run-fails
fix_status: fixed
retest_status: pending
fix_commits: d2490f96e; 430f0be1c6153d4cc691038a8e088c6696707d02; 7dc8d82adcdcaa6da5780370c0d92840f3ced5dd
evidence:
last_report: docs/qa/reports/2026-09-01-profile-extension-implementer.md
overlaps: LP-003; LP-goal-command-judge; ET-spec-cycle-skill-bundle
---

The conductor delegates only: it may inspect task state and dispatch bounded workers, but it does
not edit production files. The public walk must prove spawn, blocking prompt, on-disk completion,
and stop for each worker, including exact `agent_name`, Agent-local sentinel visibility, provider,
model, reasoning effort, and speed when supplied. Prior optional-runtime evidence does not prove
the custom implementer path.

2026-08-28: `blocked-verify` — the deterministic daemon harness proves custom Agent identity,
Agent-local sentinel visibility, category runtime propagation, settlement, and worker cleanup, but
authorized provider credentials were unavailable for the isolated public-interface walk. A human
must authorize provider access and run this charter before the scenario can become `pass`. The
existing optional-runtime retest remains `pending`; no blocked result is promoted to pass.

2026-09-01: `blocked-verify` — typed entity validation now resolves the Profile lens and the direct public catalog regression passes without leaking to the default Profile. The new stock implement-tasks E2E installs `engineer` and its Agent-local skill in `engineering`, but its conductor sandbox currently exits 69 before worker spawn and the run remains active until timeout, even on the combined A+B+C head. The executable failing regression is retained; publication must not claim the full Profile engineer journey until this remaining public-path failure is diagnosed.
