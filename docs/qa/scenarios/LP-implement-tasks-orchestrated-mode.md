---
id: LP-implement-tasks-orchestrated-mode
area: LP
title: Delegate implement-tasks through orchestrated mode
persona: Bruno
journey: J-01
expected: Running implement-tasks with mode=orchestrated and implementer=custom_implementer uses the bundled orchestrator in one continuous Goal session, starts every worker with that exact Agent and its Agent-local sentinel skill, gives every task its category-selected runtime, proves completed task frontmatter on disk, stops every spawned worker, marks the per-task branch not_taken, and settles done. Omitting implementer selects code_implementer.
entry_points: compozy loop run --name implement-tasks --input slug=<slug> --input mode=orchestrated --input implementer=custom_implementer; compozy loop status; compozy session list --parent <goal-session> --agent custom_implementer; web /loop-runs/:run_id detail
qa_status: pass
bug_ids: BUG-20260826-optional-runtime-run-fails
fix_status: fixed
retest_status: passed
fix_commits: 16096e1e3f40ca9c6df7d6100e7aee28058880bf; d4df0df81d36364ed21009d68324dd6c9c2dbeaa; 5cc860834d63d2aaf2f8e68e08fce0747f7b4fc1
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

2026-09-01: `pass` — typed entity validation resolves the acting Profile, and exact daemon-issued Agent identity plus nested session commands preserve that Profile without widening other CLI namespaces. Secret-safe sandbox diagnostics retained the red `compozy me` boundary (exit 69, session not found). The green public E2E proves conductor success, three engineer workers, Agent-local sentinel visibility, ordered task completion, stopped settlement, and zero surviving workers.
