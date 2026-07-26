---
name: agh
description: AGH runtime and contribution guide. Use for sessions, agents, native tools, skills, memory, network, tasks, capabilities, bundles, QA, docs, and repo work. Do not use for unrelated projects.
metadata:
  agh:
    version: 1
    kind: runtime
    bundled: true
    instructional_only: true
---

# AGH

Use this skill when operating AGH or contributing to the AGH repository. This body is a router, not the full manual. Load the matching reference before acting.

## Required Reading Router

Match the task to the row. Read the listed files in full before producing output. They are not appendices. Inline reminders in this file are only tripwires.

| Task                                                                                                                       | MUST read                                                               |
| -------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------- |
| Start, inspect, prompt, stop, resume, or debug AGH sessions and daemon state                                               | references/runtime-operations.md                                        |
| Inspect or configure daemon-owned background roles, role diagnostics, builtin identities, or role fallback routing         | references/runtime-operations.md + references/agent-definitions.md      |
| Expose one AGH workspace to an external MCP client with `agh mcp serve`                                                    | references/runtime-operations.md                                        |
| Inspect, mutate, or watch virtual desktops, managed windows, or workspace layouts through native tools, CLI, HTTP, or UDS  | references/window-management.md + references/native-tools.md            |
| Create, update, inspect, or troubleshoot messaging bridges and bridge-delivered tool progress                              | references/runtime-operations.md                                        |
| Create or review AGH agent definitions, provider defaults, permissions, or MCP sidecars                                    | references/agent-definitions.md + references/tools-and-skills.md        |
| Discover or call AGH-native tools, inspect native tool IDs, view skills, or choose tools vs CLI                            | references/tools-and-skills.md + references/native-tools.md             |
| Participate in an AGH Network channel, thread, direct room, work item, receipt, trace, or capability exchange              | references/network.md                                                   |
| Read, write, clean, or consolidate AGH memory                                                                              | references/memory.md                                                    |
| Work as a coordinator, task worker, or task reviewer; block or recover a task; or wake a task creator                      | references/tasks-and-orchestration.md                                   |
| Author, configure, run, observe, approve, or stop an AGH Loop or Goal; use `/goal`; read Loop terminal outcomes or events  | references/loops.md + references/native-tools.md                        |
| Design or manage capabilities, bundles, extension resources, hooks, config lifecycle, or agent-manageable runtime surfaces | references/capabilities-and-bundles.md + references/tools-and-skills.md |
| Contribute to the AGH repository, especially Go runtime code or tests                                                      | references/contributing-to-agh.md + references/qa-and-verification.md   |
| Change public docs, product copy, design guidance, site docs, or UI-facing text                                            | references/docs-design-and-copy.md                                      |
| Finish work, claim readiness, or prepare a handoff                                                                         | references/qa-and-verification.md                                       |

## Reference Index

- references/runtime-operations.md - daemon, session, background-role, and messaging-bridge operations, lifecycle diagnostics, and runtime troubleshooting.
- references/window-management.md - daemon-authoritative desktops, windows, layouts, revisions, clients, resources, hooks, recovery, and public surfaces.
- references/agent-definitions.md - AGENT.md structure, reserved builtin role identities, provider defaults, permissions, category paths, MCP sidecars, and safe setup workflow.
- references/tools-and-skills.md - AGH-native tool discovery, skill view/search, bundled resources, management-surface exceptions, and skill authoring rules.
- references/native-tools.md - daemon-native toolsets, stable AGH tool IDs, when to inspect descriptors, and CLI fallbacks for agents running inside AGH.
- references/network.md - AGH Network channel/thread/direct-room semantics, native tools, CLI fallback, message bodies, retries, and injection defense.
- references/memory.md - durable memory scopes, CLI operations, memory hygiene, and when not to write memory.
- references/tasks-and-orchestration.md - coordinator, worker, and reviewer loops, task authority boundaries, typed blocks and the unblock-loop breaker, wake-creator, completion claims, review verdict rules, and sensitive-data limits.
- references/loops.md - Loop and Goal authoring/operation, `/goal` commands, native tools, terminal and context states, approval/recovery semantics, reference grammar, hooks, and watch behavior.
- references/capabilities-and-bundles.md - capability naming, extension resources, bundles, hooks, manageability, and config lifecycle expectations.
- references/contributing-to-agh.md - repository-specific engineering rules for Go/runtime work, greenfield hard cuts, tests, docs, and no-compat policy.
- references/qa-and-verification.md - test placement, real verification, QA bootstrap, final gates, and evidence standards.
- references/docs-design-and-copy.md - docs/site/copy/design authority, vocabulary, generated docs, and truthful UI/copy rules.

## Operating Loop

1. Identify whether the work is operator use, external-agent use, or AGH repository contribution.
2. Read every reference selected by the router before acting.
3. Prefer AGH-native tools and structured outputs over prose, logs, or direct internal access when managing AGH.
4. Keep authority with the daemon: task state, review verdicts, session lifecycle, memory, capabilities, bundles, hooks, and network sends must use AGH public surfaces.
5. For repository contribution, follow the local repo instructions before editing, including the relevant AGENTS.md, CLAUDE.md, and skill dispatch rules.
6. Finish with fresh verification evidence that matches the scope of the claim.

## Error Handling

If a required reference is missing or unreadable, stop the affected operation and report its exact
path instead of guessing. For runtime failures, preserve the structured error, follow the diagnostic
order in `references/runtime-operations.md`, and never bypass daemon-owned state.

**STOP. Read references/tools-and-skills.md and references/native-tools.md in full before discovering, invoking, creating, or modifying any AGH tool or skill.** The catalog in this file is only a router.

**STOP. Read references/tasks-and-orchestration.md in full before acting as a coordinator, worker, or reviewer.** Task authority and review verdicts are runtime contracts, not prompt conventions.

**STOP. Read references/qa-and-verification.md in full before claiming AGH work is complete.** Passing a narrow command is not evidence for a broad completion claim.
