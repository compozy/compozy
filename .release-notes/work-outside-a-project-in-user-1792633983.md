---
title: Work outside a project in User
type: feature
---

Sessions, Loops, Goals, terminals, tasks, and automations can now run in **User**, a scope for work
that belongs to no project. Each profile has its own User scope. It runs in your home folder (set
`[user_scope] work_dir` to change it), loads your personal configuration, and never registers a
project. Pick **User** in the workspace switcher, between **All projects** and your projects, to see
only User work on its own desktop.

- CLI: `compozy session new --user`, and `--workspace @user` on every command that takes
  `--workspace`. HTTP/UDS: `POST /api/sessions` with `{"scope":"user"}`, `@user` in workspace paths and
  selectors (`/api/workspaces/@user/...`, `workspace_id=@user`). Native tools accept
  `"workspace": "@user"`. Session payloads add `scope` and `effective_workspace_access`.
- The new AGENT.md field `workspace_access: unrestricted` lets one agent read and act on every project,
  User, and every profile, including All projects aggregates. Only the operator can set it; agents
  that try get `workspace_access_operator_only`, and marketplace or extension agents cannot ship it.
  Sessions started by other agents never get more than their creator. Tool approvals still follow
  `permissions`. Every allowed decision is logged as `workspace.access_granted` with source
  `agent_grant`, and granted sessions show an **Unrestricted** badge. This deliberately reintroduces a
  grant for this one field, superseding the earlier "no grant vocabulary" rule for cross-workspace
  access.
- New errors: `session_create_scope_conflict`, `user_scope_unsupported`,
  `user_scope_worktree_unsupported`, `user_scope_work_dir_invalid`, `workspace_name_reserved`,
  `workspace_access_operator_only`. Registering `COMPOZY_HOME` now fails like the home folder
  (`workspace_home_forbidden`). User work has no worktrees.

Migration notes: the first start moves the hidden `COMPOZY_HOME` project (and any leftover
home-folder registration) into the User scope of each profile that owns its work. Tasks, automations,
Loop judges, and managed Goals that ran there now run in User; automations that target a project Loop
keep running in that project. Session identities and creation stamps are unchanged. Terminal history
and retained recordings are copied per profile and verified before the old journal folder is renamed
with `.migrated`; keep those folders for recovery. Layouts are copied when no User layout exists yet.
The hidden project is removed only after every store finishes and no row still references it; an
interrupted upgrade resumes on the next start. A hidden project with worktrees, or targeted by a
Loop automation, stays an ordinary project, addressable by its ID or name, and the log says so.
Nothing is deleted: colliding rows and project-only data (coordination, extension bindings and dev
links, model discovery contexts) are kept in `user_scope_migration_archive` with their original JSON;
`user_scope_migrations` records progress. Projects named `user`, `global`, or starting with `@` are
renamed (leading `@` stripped, `-project` appended when needed); the old name keeps working with a
deprecation warning through v0.4.0 and is removed in v0.5.0. `@user` always means User; on window
surfaces `global` always means the All projects desktop. Daemons older than v0.4.0 reject `AGENT.md`
files that contain `workspace_access`, so upgrade every daemon that reads them before adding it.
Records saved before this release carry no grant ceiling; save them again as the operator to let a
granted agent's tasks, automations, or Loop runs keep the grant.
