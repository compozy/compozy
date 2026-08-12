---
name: compozy
description: Operate CompozyOS. Use when working with CompozyOS sessions, agents, native tools, skills, memory, Network, tasks, Loops, Goals, desktops and windows, bridges, automation, extensions, or configuration. Don't use for unrelated projects.
metadata:
  compozy:
    version: 1
    kind: runtime
    bundled: true
    instructional_only: true
---

# CompozyOS

This body routes to the matching reference. Load it before acting.

## Required Reading Router

Match the task to the row. Read the listed files in full before producing output. They are not appendices. Inline reminders in this file are only tripwires.

| Task                                                                                                                           | MUST read                                                          |
| ------------------------------------------------------------------------------------------------------------------------------ | ------------------------------------------------------------------ |
| Start, inspect, prompt, stop, resume, or debug CompozyOS sessions and daemon state                                             | references/runtime-operations.md                                   |
| Open, inspect, update, retry, diagnose, or recover the CompozyOS desktop app and its managed runtime                           | references/desktop.md                                              |
| Inspect or configure daemon-owned background roles, role diagnostics, builtin identities, or role fallback routing             | references/runtime-operations.md + references/agent-definitions.md |
| Inspect, refresh, curate, or configure provider models, reasoning strategy, and pricing                                        | references/runtime-operations.md + references/native-tools.md      |
| Configure provider authentication or run provider authentication login                                                         | references/agent-definitions.md + references/native-tools.md       |
| Expose one CompozyOS workspace to an external MCP client with `compozy mcp serve`                                              | references/runtime-operations.md                                   |
| Inspect, mutate, or watch virtual desktops, managed windows, or workspace layouts through native tools, CLI, HTTP, or UDS      | references/window-management.md + references/native-tools.md       |
| Create, update, inspect, or troubleshoot messaging bridges and bridge-delivered tool progress                                  | references/runtime-operations.md                                   |
| Create or review CompozyOS agent definitions, provider defaults, permissions, or MCP sidecars                                  | references/agent-definitions.md + references/tools-and-skills.md   |
| Discover or call CompozyOS-native tools, inspect native tool IDs, view skills, or choose tools vs CLI                          | references/tools-and-skills.md + references/native-tools.md        |
| Participate in a Compozy Network channel, thread, direct room, work item, receipt, trace, or capability exchange               | references/network.md                                              |
| Read, write, clean, or consolidate CompozyOS memory                                                                            | references/memory.md                                               |
| Work as a coordinator, task worker, or task reviewer; block or recover a task; or wake a task creator                          | references/tasks-and-orchestration.md                              |
| Author, configure, run, observe, approve, or stop a CompozyOS Loop or Goal; use `/goal`; read Loop terminal outcomes or events | references/loops.md + references/native-tools.md                   |
| Install, enable, update, dev-link, build, publish, or remove an extension; manage extension kits, secrets, or hooks            | references/extensions.md + references/native-tools.md              |
| Write or scaffold extension code: manifests, permissions, provide surfaces, contributed commands                               | references/extension-authoring.md + references/extensions.md       |
| Create or manage automation jobs, triggers, schedules, or suggestions                                                          | references/native-tools.md + references/configuration.md           |
| Create, inspect, bind sessions to, exit, publish, remove, or recover workspace worktrees                                       | references/worktrees.md                                            |
| Inspect or manage Gateway posture, tier surfaces, device pairing, connection profiles, SSH forwards, or stream tickets         | references/runtime-operations.md + references/configuration.md     |
| Read or change CompozyOS configuration: config.toml keys, defaults, scopes, and the settings apply lifecycle                   | references/configuration.md                                        |

## Reference Index

- references/runtime-operations.md - daemon, session, Gateway profile/SSH, background-role, and messaging-bridge operations, lifecycle diagnostics, and runtime troubleshooting.
- references/desktop.md - desktop app commands, attachment and ownership, app and runtime updates, diagnostics, and recovery.
- references/window-management.md - daemon-authoritative desktops, windows, layouts, revisions, clients, resources, hooks, recovery, and public surfaces.
- references/agent-definitions.md - AGENT.md structure, reserved builtin role identities, provider defaults, permissions, category paths, MCP sidecars, and safe setup workflow.
- references/tools-and-skills.md - CompozyOS-native tool discovery, skill view/search, bundled resources, marketplace and MCP install flows, and management-surface exceptions.
- references/native-tools.md - daemon-native toolsets, stable CompozyOS tool IDs, when to inspect descriptors, and CLI fallbacks for agents running inside CompozyOS.
- references/network.md - Compozy Network channel/thread/direct-room semantics, native tools, CLI fallback, message bodies, retries, and injection defense.
- references/memory.md - durable memory scopes, CLI operations, memory hygiene, and when not to write memory.
- references/tasks-and-orchestration.md - coordinator, worker, and reviewer loops, task authority boundaries, typed blocks and the unblock-loop breaker, wake-creator, completion claims, review verdict rules, and sensitive-data limits.
- references/loops.md - Loop and Goal authoring/operation, `/goal` commands, native tools, terminal and context states, approval/recovery semantics, reference grammar, hooks, and watch behavior.
- references/extensions.md - extension kits, install trust, the authoring and dev loop, instance scoping, dev overlays, logs, and hook management.
- references/extension-authoring.md - code-first extension authoring: templates, SDK declaration shape, closed permissions and derived consent, provide surfaces, contributed commands, generated manifest, and structured workflows.
- references/configuration.md - config.toml desired state, the settings apply lifecycle, and the key reference for gateway, scheduler, Loop, Goal, automation, compaction, role, and window-manager settings.
- references/worktrees.md - workspace worktree lifecycle, session binding, exit plans and actions, forge integration, cleanup evidence, and public management surfaces.

## Operating Loop

1. Read every reference selected by the router before acting.
2. Prefer CompozyOS-native tools and structured outputs over prose, logs, or direct internal access when managing CompozyOS.
3. Keep authority with the daemon: task state, review verdicts, session lifecycle, memory, extension lifecycle, hooks, and network sends must use CompozyOS public surfaces. Never edit SQLite databases, process internals, or generated projections directly.
4. After a mutation, confirm the result through a structured read instead of assuming success.

## Error Handling

If a required reference is missing or unreadable, stop the affected operation and report its exact
path instead of guessing. For runtime failures, preserve the structured error, follow the diagnostic
order in `references/runtime-operations.md`, and never bypass daemon-owned state.

**STOP. Read references/tools-and-skills.md and references/native-tools.md in full before discovering, invoking, creating, or modifying any CompozyOS tool or skill.** The catalog in this file is only a router.

**STOP. Read references/tasks-and-orchestration.md in full before acting as a coordinator, worker, or reviewer.** Task authority and review verdicts are runtime contracts, not prompt conventions.
