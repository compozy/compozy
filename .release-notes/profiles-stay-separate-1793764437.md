---
title: Profiles stay separate across windows, routes and reads
type: fix
---

A review of how profiles are kept apart found and closed several gaps.

- **Switching profiles closes the other profile's windows.** A session, terminal, agent or automation window opened under **All profiles** no longer stays on screen with "This session belongs to …" after you switch into one profile. It leaves the desk without stopping anything, so another profile's terminal keeps running, and you can reopen the window.
- **Agents open in their own profile.** Opening an always-on agent owned by another profile from All profiles no longer shows "Agent not found". Its window, **New session** and **Duplicate** use the agent's profile.
- **Routes check the session's profile.** Project session routes (prompt, transcript, stream, usage and the rest) now return 404 for a session owned by another profile when you select a profile. `all_profiles=true` allows reads and refuses writes with `profile_selection_conflict`. Without a selector, a route still uses the session's own profile.
- **Agent-instance writes refuse `all_profiles=true`.** Pause, resume, check, archive and the other instance writes can no longer reach another profile's instance through the aggregate.
- **One rule for `all_profiles` reads.** Terminal lists, details, streams and downloads, agent-instance lists and details, and the native tools `compozy__agent_list`, `compozy__agent_instance_get`, `compozy__agent_instance_list` and `compozy__agent_activity` accept `all_profiles` the same way, with rows labeled by `profile_name`. The instance list no longer returns 422.
- **The command palette sees only your profile's clients.** A palette call can no longer list, read or send commands to a browser tab open on another profile. `compozy cmd-palette clients --all-profiles` and `GET /api/cmd-palette/clients?all_profiles=true` still list every profile.
- **Renaming a profile keeps row actions working.** Rename, archive, Later and Move to profile on that profile's sessions no longer fail with `profile_not_found` until a reload.
- **An archived profile no longer stops the daemon from starting.** The daemon used to exit with `start provider usage: profile "<name>" is archived`, which left the desktop app on its boot error screen.

### Migration notes

Scripts that called project session routes with `profile=<other>` for a session owned by a different profile now get 404. Name the owning profile, leave the selector out, or use `all_profiles=true` for reads. `GET /api/cmd-palette/clients` now defaults to your profile; pass `all_profiles=true` for the old list.
