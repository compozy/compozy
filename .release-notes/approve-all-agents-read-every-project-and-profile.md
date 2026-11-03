---
title: Approve-all agents read every project and profile
type: feature
---

An agent session in `approve-all` mode can now read sessions in any project, in the User scope and in any profile, and can read the All projects lists, the way you can. Before, the same calls failed with `workspace_access_denied` or `agent_scope_denied`.

- **Read by session id:** `compozy__session_status`, `compozy__session_history` and the other session tools accept just a `session_id`. CompozyOS finds where that session lives and checks access there, so the agent no longer has to pass `workspace`.
- **Other profiles and All projects:** `profile`, `all_profiles` and `all_workspaces=true` reads follow the permission mode. `approve-reads` still asks you first, and `deny-all` still never leaves its own project.
- **Secrets are redacted:** when an agent reads something outside its own project or profile, credential-shaped text in what comes back, such as API keys, bearer tokens, `password=` values and long random tokens, is replaced with `[REDACTED]`. This works by recognizing patterns, so a secret without a recognizable shape can still get through. Reads inside the agent's own project and profile don't change.
- **Clearer denials:** a refused call now says which setting refused it and what would allow it. It never names the target, so a refused agent can't tell whether a session it asked about exists.

This widens what agents can see. `approve-all` is the built-in default, so unless you changed `[permissions] mode` or an agent's `permissions`, your agents can now read work in every profile. Set `approve-reads` to be asked first, or `deny-all` to keep a session inside its project. Changing work in another profile, or writing through an All projects route, still needs `workspace_access: unrestricted`, which only you can set.
