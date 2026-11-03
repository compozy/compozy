---
title: GitHub Copilot CLI works as a full provider
type: feature
---

You can now sign in to GitHub Copilot CLI from CompozyOS, run sessions on it, and use CompozyOS native tools inside those sessions. Before this, `compozy provider auth login copilot` failed and Copilot sessions had no `compozy__*` tools. Setup steps are on the Providers docs page under "GitHub Copilot CLI".

- `compozy provider auth login copilot` runs `copilot login`. `compozy provider auth status copilot --remote` checks the sign-in with a session that sends no prompt.
- The model picker lists the models and reasoning levels Copilot offers your account. Plans with a single model, such as Copilot Free, offer no model choice: Copilot picks the model, and asking for a specific model or level stops the session before its first prompt instead of being ignored.
- Copilot ignores stdio MCP servers, so CompozyOS serves native tools to it on a local HTTP address protected by a per-session token. The address closes when the session's Copilot process exits. Any ACP provider can opt in with the new `session_mcp_transport = "http"` setting.
- `permissions.provider_full_access` now covers Copilot by turning on its `allow_all` option. Copilot's Autopilot mode and `allow_all = on` count as unrestricted, so both need `approve-all`.
- Starting a new session while its ACP provider is signed out now fails with a sign-in diagnostic naming `compozy provider auth login <provider>`, instead of a raw protocol error. This applies to every ACP provider: API and CLI errors now carry the `provider_not_authenticated` diagnostic. Resuming a stopped session still reports the provider's own error.
- The GitHub Copilot mark appears wherever CompozyOS shows the provider.

Copilot's ACP support is a public preview from GitHub, and CompozyOS was tested with Copilot CLI 1.0.105. Copilot can't run triage sessions, because it has no read-only mode. It can't take guidance during a running turn, so a message sent then cancels the turn unless you queue it. Copilot also reports no usage limits to CompozyOS.
