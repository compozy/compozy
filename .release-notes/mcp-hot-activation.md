---
title: MCP servers and extensions work in open sessions from the next message
type: feature
---

Adding, enabling or signing in to an MCP server, or installing or enabling an extension, now makes
its tools callable in the sessions you already have open, from your next message. You no longer stop,
resume or fork a session to pick up a server, and forks, continued sessions and child sessions follow
the current server set too. Removing, disabling or signing out withdraws the tools on the next call.
When a runtime can't refresh its tool list by itself, CompozyOS reloads it once before your message,
keeps the conversation, and records one quiet "MCP servers updated" line in the transcript. If the
runtime can't start, the message is not sent and goes back to the composer; send it again to retry.

Each session shows which servers it can use. The new **MCP** control in the composer, `compozy
session mcp <id>` and the `compozy__session_mcp_list` native tool list every configured server with
its state (Ready, Applies on your next message, Needs sign-in, Waiting for approval, Couldn't connect,
Not available in this session) and the action that fixes it.

**Runtime MCP servers.** Settings → MCP servers and Marketplace → MCPs now also list, read-only, the MCP
servers each coding agent CLI loads from its own files (Claude Code, Codex, Gemini CLI, OpenCode, Kimi
CLI, Qwen Code, GitHub Copilot CLI, Cursor Agent, Antigravity and Pi), grouped by runtime. CompozyOS
only reads those files: it never edits them, runs the runtime, connects to the servers or reads a
keychain. **Copy to CompozyOS** (`compozy mcp import <runtime>/<server>`) turns a runtime server into a
CompozyOS server, storing its values as Vault secrets. The new `[providers.<id>] runtime_mcp = "off"`
setting, also a switch in Settings, stops Claude Code, GitHub Copilot CLI or Qwen Code from loading
their own servers in CompozyOS sessions, from each session's next message.

New CLI verbs `compozy mcp list`, `compozy mcp allow`, `compozy mcp import` and `compozy session mcp`,
native tools `compozy__mcp_list`, `compozy__mcp_allow`, `compozy__mcp_import` and
`compozy__session_mcp_list`, and the matching HTTP/UDS routes are additive.

## Migration notes

- **Project MCP servers need approval.** A server declared in a project's files
  (`.compozy/mcp.json`, the project's `config.toml`, `.compozy/profiles/<name>/…`, or an agent or skill
  folder inside the project) now waits for your approval, bound to its exact definition, before any
  session can call it. Allow it with **Allow** in Settings or the session's MCP control, or
  `compozy mcp allow <server> --workspace <project>`. Editing the definition asks again. Servers you
  add through CompozyOS, servers in your own `~/.compozy` files and servers of enabled extensions need
  no approval.
- **Unapproved project servers no longer start.** Before this release, CompozyOS could start an
  unapproved project stdio server during tool discovery or a Settings status probe. A server waiting
  for approval is now never started, connected to or signed in to.
- **`external_default = "ask"` is stricter for project servers.** `tools.policy.external_default` keeps
  its values and default, but now applies only to servers without your consent. With `ask`, an
  unapproved project server used to be callable after a per-call prompt; it now waits for approval and
  is never started. Servers with consent still prompt on each call under `ask`. `enabled` still admits
  project servers without approval.
- **Session MCP limits are stored as intent.** Stored sessions and always-on instance ceilings are
  upgraded in place by an automatic database migration that records whether each one follows the
  current server set or keeps an explicit list. Nothing is lost, and archived sessions and stopped
  instances are covered. Rows written by an older binary during a downgrade window are still read
  correctly by a compatibility decoder in this release and the next one; that decoder is removed in
  the release after next. Upgrade through this release (or the next) before skipping further ahead
  with a database an older binary wrote to after this upgrade.
- The public lineage `permission_policy.mcp_servers` now reports the session's effective server list.
