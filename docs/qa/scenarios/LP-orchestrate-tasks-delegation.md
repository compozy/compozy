---
id: LP-orchestrate-tasks-delegation
area: LP
title: Deliver an authored spec through the bundled orchestrate-tasks Loop
persona: Bruno
journey: J-01
expected: The bundled orchestrate-tasks Loop is listed with its Engineering catalog entry, accepts slug plus the optional orchestrator agent input, runs one continuous Goal session that spawns and stops one named worker session per task, and closes only when the workspace-root command judge finds status completed in every .compozy/tasks/<slug>/task_*.md.
entry_points: compozy loop list; compozy loop run --name orchestrate-tasks --input slug=<slug>; compozy loop status; compozy session list --parent <goal-session>; web /loop-runs/:run_id detail; /marketplace/bundled/spec-cycle
qa_status: blocked-decision
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: LP-goal-command-judge; ET-spec-cycle-skill-bundle
---

The Loop delegates rather than fans out: its single Goal node conducts, and every code edit happens
inside a spawned worker session. Walking it therefore has to prove both halves — that the conducting
session never implements, and that each worker session is created, used, and stopped.

Walk points:

- The catalog lists `orchestrate-tasks` under Engineering alongside `implement-tasks` and
  `review-and-fix`, with a `use_when` that distinguishes session delegation from per-task fan-out.
- `slug` is required; omitting `orchestrator` resolves the runtime default agent `general`. The Loop
  declares no provider or model, so workers bind through the agent definition and workspace defaults.
- During the run, `compozy session list --parent <goal session id> --type spawned` shows one session
  named `orchestrate-<slug>-<task_id>` at a time, and no worker session stays active after its task
  advances, blocks, or the run ends.
- The `tasks_completed` judge executes from the workspace root: with one task file left at
  `status: pending` the criterion returns a non-zero exit and the Goal takes a revision turn; with
  every task file at `status: completed` it approves and the run reaches `done`.
- A worker that cannot finish after one corrective prompt produces `status: blocked` citing
  `.compozy/tasks/<slug>/logs/<task_id>.jsonl`, and the run still ends in a named terminal state.

QA impact 2026-08-14: new bundled Loop plus the ninth bundled skill `cy-orchestrate-tasks`.
Blocked-decision: walking this scenario spawns real worker sessions on operator provider accounts
(token spend) and consumes a sacrificial authored spec — awaiting operator approval; execution
belongs to the next release-QA lab pass.
