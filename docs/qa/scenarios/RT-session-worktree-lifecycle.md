---
id: RT-session-worktree-lifecycle
area: RT
title: Run a session inside a selected worktree
persona: Ada
journey: J-worktree-management
expected: Starting through CLI, HTTP, UDS, or native tools persists one ready-worktree binding, runs the agent and local tools inside that checkout, rejects cwd changes outside it, inherits the binding on child spawn, and exposes the same identity through filtered session reads without cross-workspace leakage.
entry_points: compozy session new --worktree|--new-worktree; compozy session list --worktree; HTTP/UDS POST /api/sessions; GET /api/sessions?worktree=; compozy__session_create.worktree|new_worktree; compozy__session_list.worktree
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-worktree-support-20260813-083057-155448-lab/qa-artifacts/qa/native-handoff-fixed-task.json; /Users/pedronauck/dev/qa-labs/compozy-worktree-support-20260813-083057-155448-lab/qa-artifacts/qa/browser-worktree-bound-context.png; internal/daemon/daemon_worktree_e2e_integration_test.go; /home/francisross/dev/qa-labs/compozy-session-worktree-reconcile-20260823-032508-181632-lab/qa-artifacts/qa/cli-runtime-evidence.json; internal/observe/reconcile_test.go
last_report: docs/qa/reports/2026-08-23-session-worktree-reconcile.md
overlaps: RT-worktree-api-surface-parity; RT-worktree-cli-lifecycle
---

QA impact: Task 03 adds structural worktree binding to session creation, containment, persistence,
spawn, and list filtering. The Phase C walk must compare the same bound session over
every structured surface and prove that sibling and parent checkout files stay outside its tool root.
It must also prove a hook cannot rewrite the resolved cwd and a child cannot select or fall back to
another checkout.

QA 2026-09-10: The existing daemon worktree integration passed real local commands after terminal authorization received the resolved worktree roots. The owning containment assertions remain enabled. See [release integration recovery](../reports/2026-09-10-release-integration-repair.md).

QA 2026-09-10 follow-up: Caller cancellation now takes precedence when ready completion and cancellation are both observable. The existing 200-repeat race regression and real worktree lifecycle integration passed. See [release integration recovery](../reports/2026-09-10-release-integration-repair.md).

Issue #640 regression: During an active worktree-bound session, use native terminal exec to start
an owned disposable lifecycle command, read bounded output and verify its reported cwd/owner,
then stop it and confirm exit. Also verify terminal open/exec with omitted and relative cwd use
that worktree. Parent/sibling cwd, workspace add_dirs, symlink escapes, foreign profiles and stale
runs must fail before process creation. Native tools and authenticated HTTP/UDS agent calls share
the same resolver; agent ACP pipe launches enforce that binding too; human terminals and system-owned pipe roots retain their existing rules.

Issue #640 verification: PASS for the isolated native terminal lifecycle: parent cwd rejected,
omitted cwd started the bound-worktree process, exact owner/root and before/after health matched,
and owner-checked stop exited with code 0 and closed the port. Public evidence is recorded in
[PR #641](https://github.com/compozy/compozy/pull/641). `TestManagerAdmissionAndScope` owns default,
relative, parent/sibling, symlink and agent-pipe allowed-root containment coverage;
`TestDaemonTerminalExecutionRoot` owns worktree-profile and native/HTTP error projection coverage.
Review follow-up results and current-head delivery gates are recorded on the same PR.

QA impact 2026-10-07 (memory removal): worktree sessions no longer share a parent workspace memory (the feature was removed); every other binding, containment, spawn, and filtering check is unchanged. No verdict change.

QA impact 2026-10-21 (spawn orchestration): a governed child may now select its checkout
explicitly. `compozy spawn --worktree|--new-worktree[=name] --base --branch`,
`compozy__session_spawn.worktree|new_worktree`, and `POST /api/agent/spawn` bind a ready worktree
of the parent's workspace (or create one) instead of inheriting the parent's binding. Without those
inputs the child still inherits. Hooks still cannot rewrite the binding. Walk: spawn a child from an
unbound parent with `new_worktree`, confirm the child's ACP cwd and provider sandbox root are the
new checkout and that a write there needs no sandbox approval. Then force a refusal (a widening
`tools` atom) and confirm the created worktree is removed. Owning automated evidence:
`TestManagerSpawnCreatesChildWithDurableLineageAndNarrowPermissions` (selected worktree subtest),
`TestDaemonNativeSessionSpawnOrchestration`, and
`TestBaseHandlersAgentSpawnBindsWorktreeAndSendsFirstPrompt`.

2026-10-27 (#992) QA impact: existing-session bind/rebind/unbind is additive. Verify idle-only
admission, unchanged session/transcript/window identity, next-turn cwd and file access, previous
process exit, and restart recovery with Claude and Codex. A missing binding still refuses resume;
explicit unbind returns to the workspace root. Session creation and conversation/worktree fork
retain their existing contracts. New worktree creation must reach ready before an existing
session can bind. Targeted runtime evidence is recorded in [the binding report](../reports/2026-10-28-issue-992-session-binding.md);
current-head browser/CI results and review dispositions are recorded in [PR #1002](https://github.com/compozy/compozy/pull/1002).
Historical passes do not validate the added path.

2026-11-01 (always-on-agent D-22) QA impact: bind and unbind a stopped ordinary
session while the held-permission work-signal source is installed. Both operations
must preserve the stopped state and must not launch a provider. The owning case is
`TestManagerIntegrationWorktreeBindingLifecycle/Should_bind_stopped_sessions_and_leave_a_removed_checkout`.
Focused integration passed with `CGO_ENABLED=1 go test -race -tags=integration ./internal/session -run TestManagerIntegrationWorktreeBindingLifecycle/Should_bind_stopped -count=1` (6.794s), with `COMPOZY_*` unset and an isolated build cache. This verifies stopped-session binding with real SQLite and worktrees; the broader scenario verdict is unchanged.
