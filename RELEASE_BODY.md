## 0.7.1 - 2026-11-15

### Bug Fixes

- Re-check a stale needs_login provider after a verified usage collection (#1697)
- Cap reasoning effort levels to the model profile (#1704)
- Stop reporting a deleted child session as working (#1693)
- Keep the Archived and Later session list views when opening a session (#1707)
- Show the session's agent avatar on session tabs (#1710)
- Stop E2E leak checks from matching Vite asset hashes (#1714)
- List unfoldered sessions beside folders in Archived and Later (#1718)
- Stop counting deleted subagent children as working (#1711)
- Open transcript terminals bound to their project and never trap their window (#1719)
- Keep session paging below the last row in folder lists (#1731)
- Bound automation initialization retries without orphan sessions (#1730)
- Bound session query quiescence before deletion
- Show active child sessions and subagents first in the session list
- Retry an unconfirmed ACP reasoning effort before failing the bind (#1681)
- Preserve concurrent ACP configuration confirmations
- Resolve live User MCP policy without a project lookup
- Let daemon boot with archived sessions missing from disk
- Wait for the model catalog before rejecting a model pin (#1689)
- Ignore redundant ACP mode notifications
- Settle failed subagent delegations and reconcile orphans at boot (#1686)
- Preserve parent wakes after recovery cancellation
- Preserve the final ACP confirmation window
- Stop the child session when a subagent is canceled (#1688)
- Wait for daemon process exit before cleaning SSH homes (#1726)
- Cancel descendants below terminal delegated sessions
- Host the rail delete confirmation at the shell (#1703)
- Preserve same-workspace instance permission intent
- Finish subagent roster deletion after caller disconnects
- Close the deleted subagent drawer after confirmation
- Avoid authority warnings for canceled session reads
- Complete subagent deletion test payloads
- Format hosted protocol fixture
- Target pending focus in window layout commands
- Preserve native tools for provenance sessions
- Retain live streams across reordered event publication
- Prevent project configuration from granting MCP consent
- Authorize extension replacement across attachment profiles
- Retry automation startup failures before provider acceptance
- Preserve session tool access when upgrading from v0.7.0
- Settle empty subagent wakes after deleting results
- Delete retained sessions with missing history files
- Preserve released subagent CLI error exits
- Narrow default spawn tools to parent authority
- Keep session deletion within source conventions
- Format subagent CLI compatibility coverage
- Correct formatting in release fixes
- Preserve stricter workspace tool policies
- Retain valid lineage in deletion identity fixture
- Retain legacy delegated instance permission ceilings
- Prepare catalog executable before timed E2E journeys
- Keep E2E lane preparation within function size limits
- Preserve unknown agent errors for stale derive previews
- Finish accepted layout changes after client disconnects

### Features

- Clean up archived session history in bulk (#1717) (#1721)
- Make MCP servers and extensions usable in open CompozyOS sessions on the next message (#1725)
- Make the client request budget configurable (#1687)
- Delete delegated subagents from their row menu (#1705)
- Replace an installed extension on install (#1692)

### Performance Improvements

- Remove idle shell subscriptions from tab switching (#1701)
- Settle the retained-window live-data lease so tab switches stop rewiring apps (#1729)
- Avoid forced layout in the Tasks toolbar

### Refactoring

- Load session dialogs through the public barrel

### Testing

- Assert Markdown Preview default in Files E2E-002 (#1699)
- Cover automation startup discard policy per attempt
- Cover CLI timeout rejection through the default loader
- Compose MCP authority and consent in integration fixtures
- Use typed external policy values in User resolver contract
- Include deleted subagents in queued wake fixtures
- Give hosted protocol scenarios one MCP client owner
- Cover active subagents and settled branch paging
- Settle MCP summary before derivation source fences
- Align CLI timeout coverage with Go conventions
- Complete MCP authority integration fixture wiring
- Bind focus intent to the controller fixture
- Capture peer convergence CPU profiles for release diagnosis
- Isolate remaining MCP integration fixtures
- Capture remaining switch CPU work in CI
- Retain provider configuration and resumed MCP launches
- Establish restored automation tab selection
- Expose consent projection failures in CI
- Persist daemon policy integration agents
- Retain switch profiles from passing CI samples
- Preserve hosted client cleanup and stopped session recovery
- Isolate the closed bridge port probe
- Remove temporary release CPU profiling
- Subscribe the consent client to catalog changes
- Profile remaining peer convergence work
- Remove temporary peer convergence profiling
- Format session upgrade regression coverage
- Wrap upgrade migration fixture queries
- Upgrade permission intent fixtures through the migration stream
- Align peer convergence profiles with long tasks
- Start peer profiling before convergence warmup
- Reuse permission intent migration prefix fixture
- Wait for persisted terminal approval decisions

### Release Notes

#### Features

##### MCP servers and extensions work in open sessions from the next message

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
- **`external_default = "disabled"` no longer blocks servers you set up yourself.** With `disabled`
  (the default), only project servers waiting for approval stay blocked. Servers you add through
  CompozyOS, servers in your own `~/.compozy` files, servers of enabled extensions and project servers
  you allowed are now callable. If you relied on `disabled` to block every external MCP server,
  disable or remove the servers you don't want sessions to use.
- **Spawned sessions inherit the parent's MCP servers and tools by default.** A child session or
  subagent spawned without an explicit MCP server or tool list now follows its parent's current
  servers and tools instead of starting with none. It never gets more than its parent. To keep a child
  narrower, name its MCP servers and tools when you spawn it.
- **Session MCP limits are stored as intent.** Stored sessions and always-on instance ceilings are
  upgraded in place by an automatic database migration that records whether each one follows the
  current server set or keeps an explicit list. Nothing is lost, and archived sessions and stopped
  instances are covered. Rows written by an older binary during a downgrade window are still read
  correctly by a compatibility decoder in this release and the next one; that decoder is removed in
  the release after next. Upgrade through this release (or the next) before skipping further ahead
  with a database an older binary wrote to after this upgrade.
- **Existing v0.7.0 sessions keep their access.** The corrective migration also covers databases that
  already applied the initial intent backfill in a development build. Empty provenance policies and
  older root budgets without materialization flags follow the current native, extension and MCP
  catalog. Forks, continued sessions and recovery sessions refresh their native tools too. Spawned
  and coordinator limits, explicit `allowed_tools` profiles, ancestor ceilings and lists authored
  after the intent upgrade remain enforced. After upgrade, legacy descendants without a spawn
  marker may gain access to additional tools and servers within their always-on instance's ceiling.
  Delegated sessions never exceed their instance ceiling.
- The public lineage `permission_policy.mcp_servers` now reports the session's effective server list.

#### Fixes

##### Automation backoff retries unaccepted provider startup failures

`retry.strategy = "backoff"` again retries ACP startup, process-exit, handshake, and protocol
failures before session acceptance, including a crash at spawn, package preparation failure, or
`session/new` error. The existing retry count and delay apply, including to repeated typed startup
configuration failures. A provider that already accepted the session is never started again by a
startup retry. Cancellation, authentication failures, errors without an ACP startup classification
(such as agent-definition validation), and cleanup that retains ownership stay terminal. Exhausted
retries preserve the final failed session and diagnostics.

##### Delete archived history whose session files are missing

Archived sessions retained after their files disappear can now be deleted explicitly or through archived history cleanup. Cleanup reports a deletion only when the catalog entry was removed or confirmed already gone.

##### Extension replacement respects profile ownership before publication

An agent can replace a local extension only when every installation belongs to its workspace and
its own profile or all profiles. An attachment in another profile requires operator authority.
Replacement also resolves the selected installation before staging, so a denied or missing target
never temporarily publishes candidate code to another installation.

##### Keep layout changes when the page reloads

Tab pins and other accepted layout changes now finish saving when the page reloads immediately after the change.

##### Project configuration cannot grant MCP consent to itself

Security change: workspace and workspace-profile configuration can only restrict inherited external
tool policy. `tools.policy.external_default` keeps the stricter value (`disabled` before `ask` before
`enabled`), and `trusted_sources` retains only entries already trusted by the preceding layer.
Omitting the trust list inherits it; an empty list removes all trust. Attempted project grants are
ignored with one warning per daemon process. Put intentional grants in user or personal profile
config. Project files and existing explicit server approvals are preserved.

##### Narrow inherited child tools to the parent agent's permissions

Spawning a less restricted child agent with default inheritance now narrows any broader effective tool set to the parent's permissions. Explicit requests for tools outside the parent's permissions are still refused.

##### Preserve subagent CLI error compatibility

`compozy session subagents` list, show and cancel keep the v0.7.0 exit code 1 for daemon errors and print each error once, including when a workspace is selected explicitly.

##### Keep subagent completion wakes flowing after deletion

Deleting the only result in a queued subagent completion now cancels the empty wake completely, so later subagents can still wake their parent.
