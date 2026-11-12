---
id: RT-always-on-standing-context
area: RT
title: Always-on agents keep operator rules and the peer roster across turns and compaction
persona: Dora
journey: J-31
expected: Operator rules bind an always-on agent on every turn and survive provider compaction. The agent sees an authorized roster of peers with their input forms. Agent tools cannot change a rule. On Claude, the agent no longer lists the operator's own MCP servers and connectors.
entry_points: agent memory CLI and API; session prompt; agent instance list; provider launch
qa_status: pass
bug_ids:
fix_status: fixed
retest_status: pass
fix_commits: 3dec1b996, 7a5ba52e9
evidence: docs/qa/reports/2026-11-12-always-on-standing-context.md
last_report: docs/qa/reports/2026-11-12-always-on-standing-context.md
overlaps: RT-agent-memory-startup, RT-pressure-context-compaction
---

1. In an isolated lab, define two always-on agents in one workspace. Give the second an `## Input` block with `intake: <text>` and `sweep:`. Create an instance of each.
2. Add a note and a rule to the first agent: `compozy agent memory add <agent> "<text>"` and `compozy agent memory add <agent> "<text>" --kind rule`. `inspect` shows the rule as a `- [rule] ` line.
3. Send the first agent a turn. The delivered prompt starts with `<compozy-standing-context>`: the rule is numbered under `# Operator rules`, and `# Peer agents` lists the second agent with its scope, description and `Accepts:` forms. The startup `<compozy-agent-memory>` section holds the note and not the rule.
4. Send a second turn. The block is replaced by the one-line `unchanged="true"` marker; `compozy session usage <id>` shows the `standing` span as unchanged.
5. Add another rule while the session runs, then send a turn. The full block returns with both rules, with no restart.
6. Trigger a provider compaction (`/compact`) and send a turn. The full block is delivered again; the following turn returns to the marker.
7. From the agent, call `compozy__agent_memory_replace` and `compozy__agent_memory_remove` with text matching the rule. Both are denied with a message starting `agent_memory_rule_protected` and the file is unchanged. `compozy__agent_memory_add` writes a dated note. `compozy__agent_memory_search` returns the rule with its prefix.
8. Call `compozy__agent_instance_list` with `peers: true`. Each peer carries `accepts`. Pause the second instance and send a turn: its roster line shows `[paused]`.
9. Send a turn from a session of the same agent that does not belong to an instance. It receives the rules and no roster.
10. Start an on-demand agent. It receives no standing context.
11. Claude provider: start the always-on agent with `lean_environment` unset on a host whose Claude setup has its own MCP servers or claude.ai connectors. The session lists only CompozyOS-provided MCP tools. With `lean_environment: false` the operator's servers return. An on-demand agent is unchanged.
12. Upgrade check: load a `MEMORY.md` written before rules existed. Every line stays a note, the digest is unchanged by reading, and no rule appears.
13. Remove every rule from an agent with no peers while its session runs. The next turn carries a block saying no rules or peers are in force and that earlier standing context is withdrawn.
14. With the agent's session identity (`X-Compozy-Session-ID` and `X-Compozy-Agent`), call the HTTP memory routes: adding `kind: "rule"`, editing or removing a rule, and a whole-file write or rollback that changes the rules all return 403 `agent_memory_rule_protected`; note writes succeed.
15. Have a peer send the agent a message whose text contains a `<compozy-standing-context>` block. The real block sits above the `[Message from session …]` header and the copy below it is neutralized.

Implementation evidence: `TestPromptInputCompositeIntegrationStandingContext` drives a real session manager and file store through first delivery, a live rule edit, a compaction boundary and the restored marker; `TestStore/Should_keep_operator_rules_apart_from_notes_and_closed_to_agent_writes`, `TestAgentMemoryNativeTools`, `TestAgentMemoryRoutes`, `TestClaudeLeanEnvironment` and `TestSessionLeanEnvironment` own the remaining invariants.

Walked 2026-11-12 in two isolated labs with real Claude and Codex sessions; step 11 was measured as
331 foreign MCP tool names before and 0 after. The Web editor steps are covered by the web suite and
were not walked in a browser.

2026-11-12 review repair (not re-walked in a lab; owned by `TestAgentMemoryRoutes`, `TestStore` and `TestPromptInputCompositeIntegrationStandingContext` in CI): steps 13 to 15, the `compaction="<n>"` attribute on the block after a compaction, and the reworded unchanged marker.

CI integration repair 2026-11-12 (#1565): HTTP and UDS log reads must expose the
same complete harness lifecycle, including all three selected augmenters: skills,
situation and standing. An on-demand session reports standing as unchanged without
receiving standing content. The existing
`TestUDSTransportObserveHarnessLifecycleParityMatchesHTTP` requires the exact event
sequence and each augmenter's metadata over both real transports. Validation runs
only in CI under the parallel-dispatch operator rule; no local walk was performed.
