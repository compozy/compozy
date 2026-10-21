---
title: Personal work runs in User without a hidden project
type: fix
---

Tasks, automations, Loop judges and managed Goals outside a project now run in the profile's User
scope. They use the configured User work directory instead of registering `COMPOZY_HOME` as a project.
Automations that target a project Loop continue running in that project.

## Migration notes

The upgrade moves the old `COMPOZY_HOME` pseudo-project and residual operator-home registrations
into User. Session identities and creation-profile stamps remain unchanged. Journal rows and retained
recordings/artifacts are copied per profile and verified before the source journal directory is
renamed with `.migrated`. Source layouts remain available; an existing User layout wins. Registration
removal occurs only after both stores finish and every workspace reference has been checked.
An interrupted upgrade resumes on the next start. Retain `.migrated` directories for recovery.

A source with a worktree or a Loop-targeting automation remains an ordinary project, addressable by
its ID or name. Logs identify these `keep_project` decisions. Colliding rows and workspace-only
extension bindings, dev links and model discovery contexts remain in
`user_scope_migration_archive` with their original JSON and source identity. Migration progress is
recorded in `user_scope_migrations`; a remaining reference keeps the registration in place.

Reserved project names (`user`, `global`, and names starting with `@`) are renamed deterministically.
The old name remains an alias with a warning through v0.4.0; aliases are removed in v0.5.0. `@user`
always selects User. A migrated `global` name selects its renamed project on data surfaces; without
such an alias it is rejected. Window-manager `global` always selects the reserved desktop partition.
