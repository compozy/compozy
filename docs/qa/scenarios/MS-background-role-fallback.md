---
id: MS-background-role-fallback
area: MS
title: Fall back background role routing before acceptance
persona: Ada
journey: J-route-background-work
expected: When a primary role route fails before acceptance, Compozy tries each declared fallback once in order (launching a route that sets command with exactly that account command), emits one correlated role.fallback.used event before each attempt carrying provider_command_fingerprint and never the raw command, and never reroutes an accepted ACP session, including one whose post-acceptance configuration failed.
entry_points: config.toml roles.<role>.fallback_chain (including route command = "CLAUDE_CONFIG_DIR=/Users/ada/.claude-work claude --acp"); compozy roles show auto_title (command column) and -o json (command, command_fingerprint); eligible coordinator, dream, extractor, auto-title, or checkpoint-summary invocation; compozy logs --workspace <ref> --session <parent-session-id> --type role.fallback.used --last 10 -o json; GET /api/logs?workspace_id=<id>&session_id=<parent-session-id>&type=role.fallback.used&limit=10
qa_status: pass
bug_ids: BUG-20260724-inherited-role-provider-resolution; BUG-20260928-route-command-env-prefix-not-launched
fix_status: fixed
retest_status: pass
fix_commits: uncommitted (task_08 part B1)
evidence: docs/qa/evidence/2026-09-28-session-continue-fork-b1/roles-show.txt; docs/qa/evidence/2026-09-28-session-continue-fork-b1/roles-show.json; docs/qa/evidence/2026-09-28-session-continue-fork-b1/role-fallback-logs.json; docs/qa/evidence/2026-09-28-session-continue-fork-b1/role-fallback-logs-http.json; docs/qa/evidence/2026-09-28-session-continue-fork-b1/role-fallback-accepted-logs.json
last_report: docs/qa/reports/2026-09-28-session-continue-fork-exec-b1.md
overlaps: MS-background-role-routing; MS-inspect-background-role-routing
---

QA impact 2026-07-23: ordered role fallback is new behavior. Planning flag only; the next QA cycle
owns successful advance, exhaustion cleanup, durable event correlation, and the no-fallback-after-
acceptance fence.

Planning 2026-07-24 (Task 05): entry points repaired for runtime truth — the memory controller has
no live LLM invocation in the current runtime (its fallback chain is a config-only seam, Task 02
evidence), so it is not an eligible fallback surface; the correlated `role.fallback.used` records
are cross-session runtime logs — not a session-events feed — read via `compozy logs` or
`GET /api/logs` filtered by type, with workspace_id and parent session_id correlation preserved
on every record. Session charter: CH-role-fallback-boundary.

QA 2026-07-24: a real auto-title primary failed before acceptance, the configured `codex/gpt-5.6-sol` fallback completed the work, and CLI/HTTP returned the same single correlated `role.fallback.used` event. After fixing the primary inherited-provider chain, the same workflow completed on `codex/gpt-5.6-luna` with zero fallback events. Ordered exhaustion, zero residue, and the post-acceptance fence passed in the real-daemon integration lane; public post-acceptance fault injection was explicitly skipped because no supported surface can kill only the accepted hidden ACP child.

QA impact 2026-07-24 (final review remediation): structured root `[roles]` mutations can now carry
ordered fallback chains through CLI and native config surfaces. The next QA cycle owns this new
mutation path; prior runtime fallback evidence remains historical.

QA impact 2026-07-25 (Roles panel redesign): the Web fallback-chain editor now edits each route
through one `RuntimeSelector` (provider + model + reasoning in a single control) instead of two text
fields and a select, and reports one "Choose a provider and model." error per incomplete route. The
daemon fallback contract is unchanged; the next QA cycle owns the new editor's add/remove, ordering,
and invalid-route focus recovery.

QA impact 2026-09-28 (fallback-account task_01): a fallback route may now carry `command` (the
route's account). Walk the route-command entry point: append

```toml
[[roles.auto_title.fallback_chain]]
provider = "claude"
model = "haiku-4-5"
command = "CLAUDE_CONFIG_DIR=/Users/ada/.claude-work claude --acp"
```

then confirm `compozy roles show auto_title` prints the `command` column (and `-o json` carries
`command` plus `command_fingerprint`), make the primary refuse `session/new` so the auto-title child
launches with the route command, and read `compozy logs --type role.fallback.used --last 1 -o json`:
attempt 1 carries `provider_command_fingerprint` (`sha256:`) and no command text anywhere in the
event, start logs, or exhaustion error. Acceptance is now one structured fact
(`acp.AcceptedStartError`): an attempt whose `session/new` succeeded but whose later configuration
failed stops the chain. The memory controller is a live consumer again (tiebreaker, unless
`memory.controller.mode = "rules"`), superseding the 2026-07-24 config-only note. Status reset to
`untested`; task_04/05 own the re-walk.

Planning 2026-09-28 (session-continue-fork task_07): fallback-account tasks 01+02 ship on the
`continue-fork` branch as the D8 prerequisite, so session-continue-fork task_08 owns this walk (the
fallback-account task_04/05 owners are not on this branch). Plan: `docs/qa/reports/2026-09-28-session-continue-fork-plan.md`.

## 2026-09-28 walk (task_08 part B1, route-`command` leg) — FIXED

Ada, Network Tour, isolated lab `compozy-session-continue-fork-b1-20260929-010817-219689-lab`, acpmock. `roles.auto_title` primary `unreachable-title` (`/missing/…`) with one fallback route
`provider acpmock-seat`, `model fallback-title-model`, `command = "QA_ACCOUNT=title-seat <acpmock-driver … auto_title_fixture.json>"`.
`roles show auto_title` prints the Command column; `-o json` carries `command` and `command_fingerprint` (= sha256 of the command).
The role title arrived (`Checkout Retry Fencing`) only after BUG-20260928-route-command-env-prefix-not-launched was fixed (the same
launcher defect as the seat walk). One `role.fallback.used` (attempt 1, `provider_command_fingerprint`, no command text) through
`compozy logs --workspace … --session … --type role.fallback.used` and `GET /api/logs`; the command text appears nowhere in the daemon log.
Accepted-then-failed start: chain `[rejected-title-model (reject_set), fallback-title-model]` → exactly one attempt (`rejected-title-model`),
no attempt 2, title unchanged.
