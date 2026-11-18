# Runtime MCP inventory

`Reader.List` inspects native configuration files for Claude Code, Codex, Gemini CLI,
OpenCode, Kimi CLI, Qwen Code, Copilot CLI, Cursor Agent, Antigravity and Pi. It never
executes a runtime, connects to an MCP endpoint or reads credential stores. Authentication
is always `unknown`. Environment and header values stay inside the reader; public rows
contain their key names and definition digests. `target` contains only the executable
basename for stdio or the remote URL host. It excludes arguments, directories, userinfo,
paths, ports, query strings and fragments.

## Context and consumers

Settings supplies the operator home and the selected workspace root. Session consumers
supply the actual execution CWD and worktree root. `ProviderContext` accepts a resolved
provider configuration and uses the same read-only home-policy helpers as launch,
including the Pi harness for derived providers. The helper creates no directories.

`ReadContext.Profile` isolates Compozy profile caches. `RuntimeProfile` selects a native
Codex profile file and is independent of the Compozy profile. Without an explicit native
selection, Codex's `profile` config field chooses its native profile file. Claude local
entries use the main Git root for linked worktrees; project files still follow the execution
CWD. Each runtime retains its own source precedence and ancestor walk.

The daemon owns the context-keyed cache and observes discovered files, missing candidate
paths and plugin cache directories through its existing declaration watcher. Cache entries
expire after 45 seconds. File changes invalidate entries; incomplete or unreadable snapshots
retain the last successful activation digest rather than requesting a destructive reload.

The daemon constructs its inventory with `NewReader` and the installation key from
`<COMPOZY_HOME>/mcp-digest.key`, shared with approvals, consent and OAuth. A missing or
unreadable key fails construction. Definition and activation digests use keyed HMAC-SHA-256;
recreating the daemon with the same home preserves them. The zero-value `Reader` is only
for standalone consumers without persisted identities: it uses a process-local random key,
so its digests must never be stored or compared across processes.

## Safety and activation

Symlinked configuration files are resolved, then acquired through
`fileutil.ReadRegularFile` with an 8 MiB limit. Invalid, oversized, unavailable or unsupported plugin
bundle files produce a runtime-local `read_state=unreadable` with a sanitized error. Other
runtime groups remain available. Claude's exclusive `managed-mcp.json` also disables the
runtime switch in its inventory group.

`Server.DefinitionDigest` hashes the effective definition, including private values.
`Server.ActivationDigest` additionally includes runtime enablement, approval and source
identity. Snapshot activation digests include the effective rows and managed-switch
availability, but exclude unrelated configuration fields. Activation digests are not
serialized. Session admission compares them before the hosted-catalog refresh decision for
runtimes without native hot reload and for per-name `off` implementations.

Cursor approval keys follow the installed 2026.09.26 implementation: SHA-256 of
`JSON.stringify({path: projectRoot, server: parsedServer})`, truncated to 16 hexadecimal
characters and prefixed by the name. Definitions requiring process-environment expansion
remain pending because this inventory does not resolve arbitrary process secrets.

## Copy boundary

`Copyable` is conservative: only exactly representable stdio and HTTP definitions qualify.
Runtime-owned/plugin/managed sources, other transports, CWD, header helpers, environment
indirection, tool filters, URL queries/credentials and custom OAuth settings are blocked.
Unknown behavior fields are blocked as well. `Reader.Resolve` rereads one effective definition
inside the daemon copy path and rejects a changed or removed row from the last inventory.
Its opaque result exposes safe metadata and a copy definition only to the Settings importer.
Raw values never enter snapshots, DTOs or the browser. Settings owns Vault writes,
Automatic target selection, workspace approval recording and commit-aware secret cleanup.
Copying never changes native runtime files or transfers runtime OAuth credentials.

The inventory API is `GET /api/settings/runtime-mcp-servers` on HTTP and UDS. The CLI is
`compozy mcp list`; the native tool is `compozy__mcp_list`. The session read model uses `GET /api/workspaces/{ws}/sessions/{id}/mcp`,
`compozy session mcp <id>` and `compozy__session_mcp_list`. It joins the complete scoped
CompozyOS catalog with cached callable tools, consent, authority and auth state. Reads
never discover tools or probe servers. A bound session reuses its accepted launch context;
other sessions resolve their own provider home and execution CWD. Only detail reads expose
`mcp_summary`; durable `session.mcp_changed` events carry summary changes. The shared specification's Impact Analysis owns the cross-surface audit;
generated OpenAPI, Go/TypeScript clients, native catalog and CLI docs ship with the inventory. End-to-end UI scenarios and release QA remain with S9.

## Session launch switch

The session manager shares the daemon inventory cache and captures the accepted launch's
provider home, environment, CWD and worktree context. Each launch rereads that context after
command resolution. `runtime_mcp = "off"` enables Claude bridge strict MCP configuration and
turns off claude.ai connectors. A managed Claude policy keeps the stored value valid but
suppresses strict configuration; its inventory note explains the unavailable switch.
Existing always-on isolation remains enabled even with `runtime_mcp = "load"`.

Copilot adds `--disable-builtin-mcps`; Copilot and Qwen append one
`--disable-mcp-server <name>` for every server in the session inventory. These arguments
are derived anew from the accepted command on each launch, including reloads. An unreadable
per-name inventory refuses an `off` launch rather than silently loading unknown servers.

A mode change or an applicable activation-digest change reloads at the next prompt, even
when the hosted proxy has already served the current tool catalog. Copilot in `load` keeps
its native hot reload. Unreadable inventory during admission retains the bound digest.
Sessions without a hosted MCP binding retain the same runtime reconciliation state.
The shared specification owns cross-surface impact tracking; S9 owns the consolidated
QA scenarios, official skill, site documentation and runtime flag verification.
