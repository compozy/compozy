# Runtime MCP inventory

`Reader.List` inspects native configuration files for Claude Code, Codex, Gemini CLI,
OpenCode, Kimi CLI, Qwen Code, Copilot CLI, Cursor Agent, Antigravity and Pi. It never
executes a runtime, connects to an MCP endpoint or reads credential stores. Authentication
is always `unknown`. Environment and header values stay inside the reader; public rows
contain their key names and definition digests.

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

## Safety and activation

Every configuration file is acquired through `fileutil.ReadRegularFile` without following
symlinks and with an 8 MiB limit. Invalid, oversized, unavailable or unsupported plugin
bundle files produce a runtime-local `read_state=unreadable` with a sanitized error. Other
runtime groups remain available. Claude's exclusive `managed-mcp.json` also disables the
runtime switch in its inventory group.

`Server.DefinitionDigest` hashes the effective definition, including private values.
`Server.ActivationDigest` additionally includes runtime enablement, approval and source
identity. Snapshot activation digests include the effective rows and managed-switch
availability, but exclude unrelated configuration fields. Activation digests are not
serialized. S5 consumes them to reconcile runtime loading.

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
`compozy mcp list`; the native tool is `compozy__mcp_list`. Launch wiring belongs to S5 and
session read-model integration to S6. The shared specification's Impact Analysis owns the
cross-surface audit; generated OpenAPI, Go/TypeScript clients, native catalog and CLI docs
ship with this slice. End-to-end UI scenarios and release QA remain with S9.
