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

QA 2026-09-05 (loop-stability): rerunning a completed Goal uses a fresh binding epoch after
terminal cleanup; live run `looprun-bcd2b1ad861f8a46` completed generation 2. The prior canceled
run's immutable next-round selection also recovers without rewriting history. Quarantined nodes
remain parked under existing policy. See the loop-stability report, cycle 4.

QA impact 2026-09-09 (#565): completion and rerun coverage now belongs to
`TestDaemonE2EImplementTasksShouldCompleteTaskJourney` in
`internal/daemon/daemon_implement_tasks_e2e_integration_test.go`. The affected walk starts a mixed
set (`done`, `complete`, `in_progress`), verifies only the unfinished task is dispatched, then reruns
with all completion aliases and verifies zero task dispatches. Orchestrated delivery must pass the
extension completion judge and stop its workers; its finished rerun must create no new workers.
The existing worktree-only per-task journey retains its assertions. A focused Goal scenario uses
the real extension judge with opposite main/worktree task states, then reverses them, proving that
approval and rejection follow the selected worktree on each evaluation. A node-level `root`
override takes precedence over the Loop worktree default. The focused re-walk passed all three
state/precedence combinations (36.782s) with `-race -p=1 -parallel=4`; evidence:
`.cache/issue-565/e2e-root-precedence.log`. Its targeted teardown again reported `clean: true`.

The importer and judge share YAML status parsing: canonical `completed`, accepted completion
aliases `complete|done|finished`, case/whitespace normalization, and explicit validation errors for
unknown or missing status. `pending|in_progress` remain executable. Unit/RPC/evaluator coverage in
`extensions/spec-cycle/{import_tasks,provider,embed}_test.go` verifies parser and judge agreement,
including quoted status values and YAML comments. The Goal result retains `complete|blocked`.

Verified 2026-09-09: all seven cases in the existing daemon suite passed (87.429s), including
both reruns, original category/Agent/Profile/worktree journeys, and selected-root approval/rejection.
Command: `CGO_ENABLED=1 GOMAXPROCS=4 mise exec -- go test -race -tags=integration ./internal/daemon -run '^TestDaemonE2EImplementTasksShouldCompleteTaskJourney$' -count=1 -timeout=10m -parallel=4 -p=1 -artifacts`.
The runner held the shared machine verification lock and retained output in
`.cache/issue-565/e2e-round4.log`. The allocator used `/private/tmp/compozy-iso-565-drmtph` and a short
`TMPDIR`; generated daemon socket paths stayed below the macOS limit. Targeted teardown recorded
`clean: true` in that root's `teardown.json`, with no survivors. The targeted race-enabled extension
suite also passed. This evidence uses real daemon/CLI/SQLite/extension processes and an ACP fixture
subprocess; it does not claim a fresh live-provider run.

A broader orchestrated-worktree attempt exposed a separate pre-existing ACP terminal-path defect
before any judge ran: `acp: create terminal: invalid terminal cwd "<home>/worktrees/001/issue-512":
outside workspace`. `internal/sandbox/local/provider.go` constructs `LocalTerminalScope` without
`AllowedRoots`; the supplied local host reaches `internal/terminal/manager_open.go` with only the
primary workspace authorized. Those paths are unchanged by #565. Reproduction: extend the existing
worktree-only journey with `mode=orchestrated` after its per-task run, using the same selected worktree
and conductor fixture. That broader journey is **not verified** by this change. Evidence is retained
in `.cache/issue-565/e2e-round3.log` and the isolated round-3 daemon artifacts; its teardown was clean.

Review remediation 2026-09-09 (#565): the selected-root scenario now also creates `per_run`
worktrees from committed task statuses while leaving opposite statuses in the primary workspace.
Before the fix, the completed per-run pack incorrectly reached `exhausted` instead of `done`
(`.cache/issue-565/e2e-per-run-red.log`, run `looprun-a62e45b5be9aa535`). The judge now leases the
existing per-run worktree using the same run/generation/node/item identity as the session binder;
it never materializes another tree. The unchanged assertions passed all five selected-root and
precedence combinations (31.057s, `-race -p=1 -parallel=4`), including per-run approval and rejection.
Evidence: `.cache/issue-565/e2e-per-run-green.log`; targeted teardown reported `clean: true` with
no survivors at `2026-09-09T17:12:37Z`. The broader terminal-path limitation above remains separate.

QA 2026-09-10: Initial ACP process registration now retains the startup context instead of applying the separate process-finalization deadline. The existing orchestrated Profile extension Agent journey passed three race-enabled repetitions, including its local skill, worker settlement, and selected-profile assertions. See [release integration recovery](../reports/2026-09-10-release-integration-repair.md).

Worker-tool regression (#588): inspect the conductor's spawn request and persisted child lineage.
Require the root system conductor to have a concrete Agent-derived delegation budget and an explicit
bootstrap tool subset within that budget. Agents without an allowlist use the native universe minus
Agent denies; explicit overrides remain narrow. Provenance-linked system children remain unseeded.
During the worker turn, invoke
native `compozy__skill_view` for required guidance; an injected skill summary alone is not proof of
callable tools. Confirm a nondelegated tool is still denied and omission still means zero tools.
Then verify task completion, the selected Agent/Profile/runtime and stopped worker cleanup as above.
