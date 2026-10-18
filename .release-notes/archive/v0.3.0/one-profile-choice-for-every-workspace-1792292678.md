---
title: One profile choice for every workspace, and agents keep their profile
type: fix
---

Profiles are shared by all workspaces, and now the selected profile is too. Switching workspaces in the Web app keeps the profile you picked, and choosing another one in any workspace changes it everywhere. **All profiles** also stays selected when you switch workspaces or reload. While **All profiles** is selected, new work is created in the last profile you picked, which the destination chip shows.

Sessions an agent creates now land in that agent's profile. Before, a session created with `compozy__session_create` from an agent in another profile went to `default` and was only visible under **All profiles**. Child sessions from `compozy__session_spawn`, `compozy__subagent_delegate`, and `compozy session new` run inside a session already stayed in the caller's profile.

- `compozy profile use` and `compozy profile create` set the choice for every workspace. `compozy profile current` reports `remembered choice` without naming a workspace.
- Settings → Profiles no longer shows **Where each profile is active**, because there is now one choice.
- Choices remembered per workspace by earlier releases are deprecated. Until v0.5.0, a CLI command run inside such a workspace still uses its choice and prints a warning, and `PUT /api/profiles/selection` with `scope: "workspace"` returns a `Warning` header. Both stop applying in v0.5.0.

Migration notes: upgrading keeps every per-workspace choice and seeds the shared choice from the one you set most recently, so the Web app opens in the profile you used last. Picking a profile with `compozy profile use`, `compozy profile create`, or the Web switcher sets the shared choice and clears the per-workspace ones. Scripts that write `scope: "workspace"` selections should send `scope: "global"` before v0.5.0.
