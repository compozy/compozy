---
id: RT-session-fallback-chain
area: RT
title: Move a work session to the next configured seat before ACP acceptance
persona: Rafa
journey: J-route-background-work
expected: When an agent declares fallback_chain and its first route is refused before ACP acceptance, the first prompt of a logical session (and a session-owned eager start) binds on the next accepting route, writes one session.fallback.used event before each fallback attempt carrying provider_command_fingerprint and never the raw command, leaves one provider_failure marker per refused route (next_action use_fallback for rate_limited/not_authenticated with a route remaining), never advances after acceptance, and resumes on the accepted route or restarts with context replay (accepted_route_missing) when that route is no longer configured.
entry_points: AGENT.md fallback_chain (or compozy agent create|update --fallback-route 'provider=…,model=…,command=…'); compozy agent info <name> -o json; compozy session new --agent <name>; compozy session prompt <id> "…"; compozy logs --session <id> --type session.fallback.used -o json; GET /api/logs?type=session.fallback.used&session_id=<id>; GET /api/sessions/{id} (runtime.effective); compozy session stop|resume <id>
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
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
