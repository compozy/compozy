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
retest_status: pass
fix_commits: 16096e1e706261c30e112995c0cbe457c27014ce; d4df0df8adbb73896b2cd33243db98f4037b00c3; 5cc860834d63d2aaf2f8e68e08fce0747f7b4fc1; be2ca774e0ea4c5f1a3aa30fe73bb9110d451735
evidence: /tmp/compozy-pr-542-worker-report.md; internal/daemon/loop_runtime_adapters_test.go; internal/session/manager_test.go
last_report: /tmp/compozy-pr-542-worker-report.md
overlaps: LP-003; LP-goal-command-judge; ET-spec-cycle-skill-bundle
---

The conductor delegates only: it may inspect task state and dispatch bounded workers, but it does
not edit production files. The public walk must prove spawn, blocking prompt, on-disk completion,
and stop for each worker, including exact `agent_name`, Agent-local sentinel visibility, provider,
model, reasoning effort, and speed when supplied. Prior optional-runtime evidence does not prove
the custom implementer path.

2026-08-28: `blocked-verify` — the deterministic daemon harness proves custom Agent identity,
Agent-local sentinel visibility, category runtime propagation, settlement, and worker cleanup, but
authorized provider credentials were unavailable for that isolated public-interface walk. At that
point, provider access and a human-run charter remained required before the scenario could become
`pass`; the existing optional-runtime retest was `pending`, and the blocked result was not promoted to pass.

2026-09-01: `pass` — typed entity validation resolves the acting Profile, and exact daemon-issued Agent identity plus nested session commands preserve that Profile without widening other CLI namespaces. Secret-safe sandbox diagnostics retained the red `compozy me` boundary (exit 69, session not found). The green public E2E proves conductor success, three engineer workers, Agent-local sentinel visibility, ordered task completion, stopped settlement, and zero surviving workers.

2026-09-02: `pass` after resetting the stale verdict. The targeted runtime E2E re-walk proved that the Profile conductor can run `session status`, `session prompt`, and `session stop` for each spawned worker before the Loop settles done.

QA impact 2026-09-04 (PR #542): Loop-managed workers now materialize the selected Agent's effective
Speed and typed ACP-option defaults before pinning the immutable creation profile. `blocked-verify`:
the controller expressly prohibits the local E2E needed to repeat the orchestrated-mode public walk.
The focused race-test evidence is recorded in the worker report and the cited canonical suites; it does
not substitute for a public-interface walk. No QA lab or runtime process was started, so teardown is not applicable.

QA impact 2026-09-04 (loop-stability): live Codex workers completed both dependent receipt tasks
and stopped, but the deterministic judge could not find `compozy` in its PATH. The evaluator now
shares the daemon-bound subprocess environment and the built-in check uses quoted `COMPOZY_BIN`.
Fresh real-provider re-walk `looprun-bcd2b1ad861f8a46` passed: one actual worker completed exact
decimal parsing, stopped, and the Loop settled `done` in generation 1; all 14 independent invoice
tests passed. Canceled-run recovery separately exposed binding-epoch allocation and generation
selection failures under investigation. Evidence: the loop-stability report, cycle 3.
