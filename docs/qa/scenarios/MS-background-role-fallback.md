---
id: MS-background-role-fallback
area: MS
title: Fall back background role routing before acceptance
persona: Ada
journey: J-route-background-work
expected: When a primary role route fails before acceptance, AGH tries each declared fallback once in order, emits one correlated role.fallback.used event before each attempt, and never reroutes an accepted ACP session.
entry_points: config.toml roles.<role>.fallback_chain; eligible coordinator, dream, extractor, auto-title, or checkpoint-summary invocation; agh logs --workspace <ref> --session <parent-session-id> --type role.fallback.used --last 10 -o json; GET /api/logs?workspace_id=<id>&session_id=<parent-session-id>&type=role.fallback.used&limit=10
qa_status: untested
bug_ids: BUG-20260724-inherited-role-provider-resolution
fix_status: fixed
retest_status: pass
fix_commits: a9a8fcad63f4354505e4c9a0701a6d0f559cc991
evidence: /Users/pedronauck/dev/qa-labs/agh-agent-roles-devtool-oss-launch-20260724-094737-758561-lab/qa-artifacts/qa/ui-live-fallback-cli.json; /Users/pedronauck/dev/qa-labs/agh-agent-roles-devtool-oss-launch-20260724-094737-758561-lab/qa-artifacts/qa/ui-live-fallback-http.json; /Users/pedronauck/dev/qa-labs/agh-agent-roles-devtool-oss-launch-20260724-094737-758561-lab/qa-artifacts/qa/inherit-provider-fix-fallback-events.json
last_report: docs/qa/reports/2026-07-24-agent-roles.md
overlaps: MS-background-role-routing;MS-inspect-background-role-routing
---

QA impact 2026-07-23: ordered role fallback is new behavior. Planning flag only; the next QA cycle
owns successful advance, exhaustion cleanup, durable event correlation, and the no-fallback-after-
acceptance fence.

Planning 2026-07-24 (Task 05): entry points repaired for runtime truth — the memory controller has
no live LLM invocation in the current runtime (its fallback chain is a config-only seam, Task 02
evidence), so it is not an eligible fallback surface; the correlated `role.fallback.used` records
are cross-session runtime logs — not a session-events feed — read via `agh logs` or
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
