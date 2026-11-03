---
title: Stays-on agents keep watch over your projects
type: highlight
---

Agents now come in two kinds. **On demand** agents work as before: they run when something starts them. **Stays on** agents hold a place in a project, or in Global, with one main session you come back to, regular check-ins, and notes that carry over between sessions.

- **Create one:** the new create screen covers both kinds, or run `compozy agent create triage --kind stays_on --prompt "Keep issues moving."` followed by `compozy agent instance create triage --workspace <project>`. The kind is fixed at creation; duplicate an agent to choose the other one. Each profile can run 10 active instances by default.
- **See them:** avatars sit above each project's sessions and in its Kanban lane header, with anyone who needs you first.
- **Check-ins:** on a schedule, the agent reviews its project in a hidden read-only session and reports only when it has something to say. Quiet checks back off, and repeated failures pause the agent (10 by default). Reports land in the main session without starting a turn, and after an absence a **While you were away** card sums up what happened. Check-ins are on for a new instance; `compozy agent instance checks` turns them off.
- **Approvals:** requests from the agent and the sessions it starts reach you through its main session. An unanswered request is denied after 12 hours by default, and the agent records a report.
- **Memory:** edit an agent's notes in the app or with `compozy agent memory`. Agents write notes on purpose; nothing is extracted automatically. Up to 200 lines or 24 KiB load at startup. User and extension agents share notes across projects, and agents defined in a repository keep their own.
- **Identity:** a display name, description and avatar now follow an agent across sessions, tasks and automations, and the agent profile collects where it works, what it does, its Check-ins, Memory and activity.

Check-ins need a provider with a confirmed read-only mode. Claude, Codex and OpenCode qualify; other providers fail those checks with `triage_unsupported`. The agent tools for instances and memory, such as `compozy__agent_instance_create` and `compozy__agent_memory_add`, are experimental for this release and may change without the usual notice.

Migration notes: nothing changes for existing agents. They stay On demand, no `AGENT.md` is rewritten, and installing an extension never starts a stays-on agent. New settings live under `[agents.stays_on]` and `[agents.heartbeat]`, with working defaults. Older versions may reject the new `AGENT.md` keys, so keep a backup before downgrading.
