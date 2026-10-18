---
id: RT-session-fallback-chain
area: RT
title: Move a work session to the next configured seat before ACP acceptance
persona: Rafa
journey: J-route-background-work
expected: When an agent declares fallback_chain and its first route is refused before ACP acceptance, the first prompt of a logical session (and a session-owned eager start) binds on the next accepting route, writes one session.fallback.used event before each fallback attempt carrying provider_command_fingerprint and never the raw command, leaves one provider_failure marker per refused route (next_action use_fallback for rate_limited/not_authenticated with a route remaining), never advances after acceptance, and resumes on the accepted route or restarts with context replay (accepted_route_missing) when that route is no longer configured.
entry_points: AGENT.md fallback_chain (or compozy agent create|update --fallback-route 'provider=…,model=…,command=…'); compozy agent info <name> -o json; compozy session new --agent <name>; compozy session prompt <id> "…"; compozy logs --session <id> --type session.fallback.used -o json; GET /api/logs?type=session.fallback.used&session_id=<id>; GET /api/sessions/{id} (runtime.effective); compozy session stop|resume <id>
qa_status: pass
bug_ids: BUG-20260928-route-command-env-prefix-not-launched
fix_status: fixed
retest_status: pass
fix_commits: uncommitted (task_08 part B1)
evidence: docs/qa/evidence/2026-09-28-session-continue-fork-b1/seat-info.json; docs/qa/evidence/2026-09-28-session-continue-fork-b1/seat-p1.txt; docs/qa/evidence/2026-09-28-session-continue-fork-b1/seat-fallback-logs.json; docs/qa/evidence/2026-09-28-session-continue-fork-b1/seat-events.json; docs/qa/evidence/2026-09-28-session-continue-fork-b1/seat-rate-events-after-remove.json
last_report: docs/qa/reports/2026-09-28-session-continue-fork-exec-b1.md
overlaps: MS-background-role-fallback
---

Planning 2026-09-28 (fallback-account task_02): new behavior. Walk with a seat that refuses
`session/new` (rate limit, missing login, or a missing CLI):

1. `compozy agent update reviewer --fallback-route 'provider=claude,model=opus-4-8,command=CLAUDE_CONFIG_DIR=/Users/ada/.claude-work claude --acp'`
   then `compozy agent info reviewer -o json` lists the route with `command_fingerprint`.
2. `compozy session new --agent reviewer`, then `compozy session prompt <id> "Review the diff on main"`
   while seat one refuses. The prompt streams on seat two.
3. `compozy logs --session <id> --type session.fallback.used -o json` shows attempt 1, phase `bind`,
   `provider_command_fingerprint` (`sha256:`), and no command text.
4. The transcript shows one `provider_failure` marker for seat one ("Provider refused route 1 · trying the
   next configured route (…)", `next_action: use_fallback`); no agent error or stop event is recorded.
5. `compozy session show <id> -o json` `.session.runtime.effective` is the accepted route.
6. A later rate limit during a turn keeps today's behavior (`retry`, session available) and writes no
   `session.fallback.used` event.
7. Stop and resume: the session loads its native ACP session on seat two. Remove the route, stop, and
   resume again: the native id is cleared, the session restarts on the primary route, and the
   `context_rebuilt` marker carries `fallback_reason: accepted_route_missing`.

Automated evidence at authoring time: session manager suites (bind, eager create, spawn, resume) and the
real-daemon `TestDaemonE2EAgentFallbackChain` integration case. task_04/05 own the walk.

Planning 2026-09-28 (session-continue-fork task_07): fallback-account tasks 01+02 ship on the
`continue-fork` branch as the D8 prerequisite, so session-continue-fork task_08 owns this walk (the
fallback-account task_04/05 owners are not on this branch). Plan: `docs/qa/reports/2026-09-28-session-continue-fork-plan.md`.
The daemon bundles an agent named `reviewer`; the walk authors its own agent (`seat-reviewer`, as in
`TestDaemonE2EAgentFallbackChain`) instead of updating the bundled one.

## 2026-09-28 walk (task_08 part B1) — FIXED

Rafa, Network Tour, isolated lab `compozy-session-continue-fork-b1-20260929-010817-219689-lab`, acpmock. Seat one = provider `acpmock-seat` whose command is `/missing/compozy-seat-one`; seat two = a route
whose command carries an account assignment (`QA_ACCOUNT=seat-two <acpmock-driver …>`, the documented `CLAUDE_CONFIG_DIR=… claude --acp` shape).
1. `agent create seat-reviewer --fallback-route …,command=…` then `agent info -o json` lists the route with `command_fingerprint`
(`sha256:ba4a0a06…`, equal to sha256 of the command). 2. First walk FAILED: seat two was launched with `QA_ACCOUNT=seat-two` as the
executable and the chain exhausted (BUG-20260928-route-command-env-prefix-not-launched, fixed). Re-walk: the prompt streamed on seat two.
3. One `session.fallback.used` (attempt 1, phase `bind`, the fingerprint, no command text). 4. One `provider_failure` marker "Provider
refused route 1 · trying the next configured route (…)" (`next_action install_cli` — the refusal was a missing CLI, so `use_fallback`
does not apply), no error or stop event. 5. `runtime.effective.provider acpmock-seat`. 6. `seat-rate-agent` (lab fixture on seat two):
a rate limit after acceptance writes no fallback event; the session stays available and, being a user session, prescribes `handoff`
(the scenario's "retry" predates the handoff offer). 7. Stop + prompt reloads the same ACP session on seat two (no new fallback event);
after `agent update --clear-fallback-chain --command <primary>` stop + prompt restarts on the primary with a `session_recovered`
marker carrying `fallback_reason: accepted_route_missing` and a `session.fallback.route_missing` log.
