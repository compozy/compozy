---
id: RT-spawn-budget-context
area: RT
title: Report the enforced spawn budget to agents
persona: Cora
journey: J-15
expected: An agent's situation context and lineage read models show only the spawn budget the daemon enforces (no unset `0` caps), `limits` names the session's governed depth and whether it can spawn, and a refused spawn names the cap that refused it (max_children, max_active_per_workspace, max_depth, or a model override without provider) with a recovery that matches it.
entry_points: situation context (startup and per-prompt); compozy__session_status; compozy__session_spawn; POST /api/agent/spawn
qa_status: passed
bug_ids: "#1046"
fix_status: fixed
retest_status: passed
fix_commits:
evidence: docs/qa/reports/2026-10-29-issue-1046-spawn-budget-context.md; internal/situation/service_test.go; internal/session/spawn_test.go; internal/daemon/native_tools_test.go
last_report: docs/qa/reports/2026-10-29-issue-1046-spawn-budget-context.md
overlaps: RT-session-spawn-wake; RT-spawn-ttl-cleanup
---

Create a user session, then a second session with `--parent` on it (provenance lineage, display
`spawn_depth: 1`, stored budget all zeros). Ask the second session to do "frontend with model A
and backend with model B in parallel" in child sessions. Confirm its context shows
`lineage.spawn_budget` as `max_children: 5, max_depth: 1` and `limits.governed_spawn_depth: 0,
can_spawn_children: true`, that it does not cite `max_children: 0`, and that both children run on
the requested models.

Ask one spawned child to spawn a governed child of its own. Confirm its `limits` show
`can_spawn_children: false` with a `spawn_blocked_reason` pointing at `compozy__subagent_delegate`,
and that a forced spawn fails with cause `max_depth 1 reached` and the subagent recovery, not the
"stop a live child" recovery.

Call `compozy__session_spawn` with `model` but no `provider`. Confirm the refusal reads
`provider is required when model is set` instead of the generic `invalid tool request`.
