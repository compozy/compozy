---
title: Session-aware slash commands
type: feature
---

Slash commands in the composer are now backed by a single daemon-owned catalog scoped to the exact session, and they work anywhere in a prompt rather than only at the start. Built-in and ACP control commands stay standalone, while a skill command can be dropped inline, repeated, and mixed with the text you already typed; the matched skill's full instructions are injected into that same turn. The same catalog is readable from the CLI, HTTP, UDS, and a native tool, so agents can discover what a session can actually run. (#311)

- `compozy session commands <session-id>` lists the catalog, and agents read it with `compozy__command_list` in the `compozy__catalog` toolset. `compozy__skill_view` accepts a source-qualified `command_id` from that list.
- The catalog groups Built-in, Agent, and Skills at the start of a prompt and shows only effective skills inline. A bare `/skill` resolves the effective winner across bundled, global, additional, workspace, and agent-local sources; extension skills use `/extension-id:skill` and marketplace skills use `/registry-id:skill`.
- What is effective respects global and workspace scope, agent activation and disable lists, runtime disable overlays, enabled and disabled extension resources, workspace and session ownership, and live resource revisions. A `session_commands_changed` stream frame refreshes only the affected session.
- Injection is bounded at 24 KB per skill and 64 KB per turn, and repeating the same skill activates it once. Invocations persist through admission, queueing, replay, transcript storage, and the UI, and queued ones are revalidated against the exact source before dispatch.
- Slash activation is limited to human operator input. Agent-authored prompts and `compozy__session_prompt` keep slash-shaped text literal, and hooks can remove an admitted invocation but never add one.
