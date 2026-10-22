---
title: See every project at once with All projects
type: feature
---

The topbar globe now turns on **All projects**, a real place to work. Sessions, Tasks, Loops and Goals,
Automations, Terminals, Worktrees, Agents, Home, and needs-you badges list every project plus User,
and each row names its project. Items open on the All projects desktop without switching projects,
New session asks which project (and, under All profiles, which profile) it belongs to, and turning
the globe off returns to the project or User exactly as you left it. Listing toolbars gain a
**Project** filter, and the session list gains **Group by** (None, Project, Profile).

- CLI: `--all-workspaces` on `loop runs`, `loop requests`, `loop list`, `task list`,
  `automation jobs|triggers|runs|suggestions`, `worktree list`, `terminal list`, and `agent list`, with
  a `PROJECT` column (`User` for User rows, `?` when unknown).
- HTTP/UDS: `all_workspaces=true` and repeatable `workspace_ids` (`@user` for User) on sessions,
  tasks, the task dashboard, the overview, and automation jobs, triggers, and runs, with their existing
  envelopes and app access. New operator-only aggregates that refuse app tokens: `/api/loop-runs`,
  `/api/loop-requests`, `/api/loops`, `/api/worktrees`, `/api/terminals` (local only),
  `/api/automation/suggestions`, and `/api/agents/aggregate`. Agents need the unrestricted grant to read
  them; others get `agent_scope_denied`. New errors: `workspace_breadth_invalid`, `workspace_reserved`,
  `window_binding_invalid`.
- Automation runs record the project of the job or trigger that started them (`workspace_id`,
  `attribution`); runs whose project cannot be established read **Unknown project**.
- Window, desktop, and layout tools accept `workspace: "global"` for the All projects desktop
  (operator only; every agent is refused).

Breaking: the needs-you bell's `workspace_label` for items without a project changes from `"Global"`
to `"User"`. It is a display string; match on `workspace_id` instead.

Migration notes: `shell.sessions.scope` is deprecated. Through v0.4.0 it stays stored and returned,
old clients may keep writing it, and each write warns and names `shell.sessions.group_by` and the
topbar globe. While `group_by` is unset, `scope = "all-workspaces"` groups the list by project; an
explicit `group_by` wins. In v0.5.0 the daemon turns `scope = "all-workspaces"` into
`group_by = "project"` when grouping is unset, drops the key with a logged notice, and removes it from
`config.toml` on the next apply; it never blocks startup. To migrate now, run
`compozy config unset shell.sessions.scope` and `compozy config set shell.sessions.group_by project`.
Automation run history is kept: older runs whose job or trigger still exists get its project
(`derived`), the rest stay `unknown`. Per-project desktops and saved layouts are unchanged; All
projects uses the profile's existing `global` desktop.
