---
id: RT-always-on-standing-context
area: RT
title: Always-on agents keep operator rules and the peer roster across turns and compaction
persona: Dora
journey: J-31
expected: Operator rules bind an always-on agent on every turn and survive provider compaction. The agent sees an authorized roster of peers with their input forms. Agent tools cannot change a rule. On Claude, the agent no longer lists the operator's own MCP servers and connectors.
entry_points: agent memory CLI and API; session prompt; agent instance list; provider launch
qa_status: skipped
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: RT-agent-memory-startup, RT-pressure-context-compaction
---

1. In an isolated lab, define two always-on agents in one workspace. Give the second an `## Input` block with `intake: <text>` and `sweep:`. Create an instance of each.
2. Add a note and a rule to the first agent: `compozy agent memory add <agent> "<text>"` and `compozy agent memory add <agent> "<text>" --kind rule`. `inspect` shows the rule as a `- [rule] ` line.
3. Send the first agent a turn. The delivered prompt starts with `<compozy-standing-context>`: the rule is numbered under `# Operator rules`, and `# Peer agents` lists the second agent with its scope, description and `Accepts:` forms. The startup `<compozy-agent-memory>` section holds the note and not the rule.
4. Send a second turn. The block is replaced by the one-line `unchanged="true"` marker; `compozy session usage <id>` shows the `standing` span as unchanged.
5. Add another rule while the session runs, then send a turn. The full block returns with both rules, with no restart.
6. Trigger a provider compaction (`/compact`) and send a turn. The full block is delivered again; the following turn returns to the marker.
7. From the agent, call `compozy__agent_memory_replace` and `compozy__agent_memory_remove` with text matching the rule. Both fail with `agent_memory_rule_protected` and the file is unchanged. `compozy__agent_memory_add` writes a dated note. `compozy__agent_memory_search` returns the rule with its prefix.
8. Call `compozy__agent_instance_list` with `peers: true`. Each peer carries `accepts`. Pause the second instance and send a turn: its roster line shows `[paused]`.
9. Send a turn from a session of the same agent that is not an instance main or descendant. It receives the rules and no roster.
10. Start an on-demand agent. It receives no standing context.
11. Claude provider: start the always-on agent with `lean_environment` unset on a host whose Claude setup has its own MCP servers or claude.ai connectors. The session lists only CompozyOS-provided MCP tools. With `lean_environment: false` the operator's servers return. An on-demand agent is unchanged.
12. Upgrade check: load a `MEMORY.md` written before rules existed. Every line stays a note, the digest is unchanged by reading, and no rule appears.

Implementation evidence: `TestPromptInputCompositeIntegrationStandingContext` drives a real session manager and file store through first delivery, a live rule edit, a compaction boundary and the restored marker; `TestStore/Should_keep_operator_rules_apart_from_notes_and_closed_to_agent_writes`, `TestAgentMemoryNativeTools`, `TestAgentMemoryRoutes`, `TestClaudeLeanEnvironment` and `TestSessionLeanEnvironment` own the remaining invariants.
