---
id: LP-loop-environment-resolution
area: LP
title: Resolve each Loop node into its declared environment
persona: Ada
journey: J-isolated-task-loop-execution
expected: A Loop node resolves every agent session and workspace-relative extension action from its ready worktree, while root, directory, per-run, precedence, and invalid-environment behavior remain unchanged.
entry_points: compozy loop create|validate --file; compozy loop configure --file|--set; compozy loop run --config-file; HTTP/UDS Loop definition, config, and run routes; compozy__loop_create|configure|run.environment; ext__spec-cycle__import_tasks
qa_status: skipped
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: qa-lab pr-519-review-fixes worktree-removal-blocked.json; tool-run-completed.json; worktree-removed-after-tool.json
last_report: docs/qa/reports/2026-10-02-untested.md
overlaps:
---

QA impact: Issue 512 makes a named Loop worktree authoritative for workspace-relative extension
actions as well as agent sessions. The targeted walk must load a task pack that exists only in the
worktree, verify worker-session attribution, and retain root/manual-call behavior as a canary.

QA completion 2026-09-01: the root-scoped canary could not see the worktree-only task pack. Both a
CLI-started run and a Web-started run with ready Worktree `wt_d7e6c291e8cae07e` completed the same
extension action and returned the task path under `project-worktree`. CLI, HTTP, Web, and runtime
evidence agreed after a fresh Web reload. The real-worktree integration suite separately verifies
that implement-tasks worker sessions retain the same Worktree id.

QA follow-up 2026-09-01: while the real extension task run was claimed and blocked inside
`ext__spec_cycle__import_tasks`, CLI removal returned `worktree_operation_in_progress`. After the
tool completed with the worktree-only task path, the same CLI removal succeeded.

Profile policy regression (#570): repeat worker creation with a Profile-scoped Agent in a
non-default Profile, using both Workspace ID and root path. Verify the same Profile layer supplies
the Agent, permissions, and runtime settings for action, pinned, and judge sessions.
An unknown or unavailable Profile must fail before session creation, even if a same-named global
Agent exists. Retain the default Profile and worktree environment canaries above.

2026-09-27 scope update: retired product surfaces were removed from this active scenario. Historical evidence remains in the dated reports; this revised contract requires a fresh walk.

2026-10-05: Deferred from this QA cycle by the user's explicit scope reduction.
Coverage remains outstanding; this skip is not a passing result. Resume from the dated
report's session matrix in a future QA cycle.
