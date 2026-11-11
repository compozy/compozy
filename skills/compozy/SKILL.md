---
name: compozy
description: Operate CompozyOS. Use when working with CompozyOS sessions, agents, native tools, skills, tasks, Loops, Goals, Terminal, desktops and windows, automation, extensions, or configuration. Don't use for unrelated projects.
metadata:
  compozy:
    version: 1
    kind: runtime
    bundled: true
    instructional_only: true
---

# CompozyOS

Use native tools and structured results for CompozyOS operations. Canonical `compozy__*` IDs are registry identities: discover with `compozy__tool_search`, inspect the live descriptor, and invoke the reference returned by the active harness. Keep canonical IDs in policy and `tool_id` inputs.

Load references through resolved `compozy__skill_view` with `{"name":"compozy","file":"references/<file>.md"}`. Do not read skill files directly or invoke the operator-only `compozy skill view` CLI from a managed session. Missing guidance never grants access; never bypass withheld capabilities through CLI, filesystem, another identity, or direct state edits. Continue independent authorized repository work if a native operation is blocked.

## Required Reading Router

Before native operational calls or tool/skill mutations, read **references/tools-and-skills.md** and **references/native-tools.md** in full. Discovery, descriptor inspection, skill/reference reads and retained-result continuation are bootstrap exceptions. Authorized provider-owned coding, research and review do not require CompozyOS manuals.

Also read the applicable references below before the operation. Paths are relative to this skill; retain the `references/` prefix in calls.

- Sessions and daemon lifecycle/debugging; MCP serve; User scope and All projects; provider models, ACP options, reasoning, Fast, pricing and usage: references/runtime-operations.md.
- Desktop app attachment, recovery and host/app updates: references/desktop.md.
- Agent definitions, provider defaults/authentication, permissions, workspace access, MCP sidecars: references/agent-definitions.md.
- Background roles, builtin identities and fallback routing: references/runtime-operations.md; references/agent-definitions.md.
- Virtual desktops, windows and layouts: references/window-management.md.
- Terminals, command execution, input and journal: references/terminal.md.
- Skill proposals, installation scope and operator review: references/skill-authoring.md.
- Explicit always-on notes/memory: references/agent-memory.md.
- Daemon Tasks: claims, leases, coordination, review and recovery; subagent delegation/status/cancel: references/tasks-and-orchestration.md.
- Loops and Goals: author, run, approve, observe, stop and `/goal`: references/loops.md.
- Extension install, build, dev, publish, remove, kits, secrets and hooks: references/extensions.md.
- Extension code, manifests, permissions, SDK declarations and palette contributions: references/extension-authoring.md; references/extensions.md.
- Config keys/defaults/scopes/apply lifecycle; Automations and suggestions: references/configuration.md.
- Gateway posture, profiles/SSH, pairing, forwards and stream tickets: references/runtime-operations.md; references/configuration.md.
- Worktree creation/binding/exit/delivery/removal/recovery; isolated delegation and subagent Git/PR facts: references/worktrees.md.
- Operator profile selection and lifecycle: references/profiles.md.
- External apps, TypeScript/Go clients and app tokens: references/app-sdk.md.

Command palette operations use `references/native-tools.md`. Tool/skill discovery, Marketplace and manual MCP management use `references/tools-and-skills.md`. Authorized CLI fallbacks are indexed in `references/cli-fallback.md`; a denied native capability does not authorize a fallback.

Read **references/tasks-and-orchestration.md in full** before exercising daemon Task authority or submitting a review verdict. A delegated assignment or role label alone grants no Task claim, lease or review authority.

## Waiting and authority

Use `compozy__monitor_create` for external waits: `kind: "pull_request"` for PR checks/reviews (armed across wakes until the PR ends), `kind: "timer"` for delays. For other commands, call `compozy__terminal_exec` with executable in `command` and arguments in `args`; on `still_running`, register `kind: "terminal"` with the returned `terminal_id`. Never substitute shell polling or provider-internal background tasks/wakeups: they cannot start a CompozyOS turn. If policy withholds the terminal/monitor tools, shell waits remain allowed. Keep doing independent work; end the turn only after delivery or when every remaining step depends on a registered monitor or subagent.

Use public surfaces for session lifecycle, Task state/reviews, extension lifecycle, hooks and tools. Never edit SQLite, process internals or generated projections directly. Confirm mutations through structured reads.

## Incomplete references and errors

For a truncated result with an artifact, resolve `compozy__tool_artifact_read`. Pass its URI unchanged as `artifact_uri`, start at `offset: 0`, then follow `next_offset` to `eof`. Decode and concatenate `data_base64` pages, reading the retained envelope's full structured `content` in the current workspace scope. Preserve redactions. A preview is not a complete read; absent artifacts or denied continuation make the reference unavailable, not permission to bypass the reader.

Retry a failed required read once at the exact path. Correct `tool_not_found`/`tool_invalid_input` first; retry `tool_backend_failed` unchanged. After the second failure, only proceed if one native call's live descriptor fully defines the operation, risk and result handling. A visible `terminal_exec` qualifies for a one-command demonstration; multi-step terminal work still requires `references/terminal.md`. Otherwise stop the affected operation and report the path. Preserve structured errors and follow `references/runtime-operations.md`'s diagnostic order.

Startup includes this router when tools and skills are enabled, the complete tool manuals when skills are disabled, and no tool router for input-only title generation.
