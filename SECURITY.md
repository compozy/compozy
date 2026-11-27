# Security

CompozyOS is a local-first runtime. Its default trust boundary is the operating-system account that runs
the daemon, not an application-level tenant boundary. Treat every process with access to that
account, `$COMPOZY_HOME`, or the daemon socket as trusted operator software.

## Report a vulnerability

Report suspected vulnerabilities privately through the repository's
[Security page](https://github.com/compozy/compozy/security). Do not put credentials, exploit payloads,
or sensitive runtime data in a public issue. Include the CompozyOS version, operating system, affected
surface, minimal reproduction, and expected impact.

## Execution isolation

`BackendLocal` is not isolation. It launches the ACP agent and its local tool host directly on the
daemon host. Workspace roots, additional roots, environment variables, and permission settings
define the process inputs and intended scope, but they do not create a container, virtual machine,
or OS security boundary. Run untrusted agents in a provider-backed sandbox whose isolation contract
fits your threat model.

## `compozy mcp serve` authority

`compozy mcp serve` is a foreground relay to an already-running daemon. Every invocation requires a
workspace and exposes an approved subset of the existing Host API as `compozy_host__*` MCP tools. The
relay binds session, task, and resource operations to that workspace and rejects
conflicting caller-supplied workspace values. It does not mint or change native `compozy__*` tool IDs.

The default stdio transport has no separate authentication step. The local process that spawns
`compozy mcp serve --workspace <workspace>` receives operator authority for the published Host API
methods in that workspace. Configure stdio only in MCP clients and agent definitions you trust with
that authority.

Each connected client is assigned an independent principal and resource source. Closing one client
removes only that client's authority and registered resources; relay shutdown removes every
remaining client-scoped registration. A disconnected client cannot retain authority through a
later connection.

HTTP transport:

- listens only on a loopback host;
- requires a bearer token for every request;
- reads the token from an environment variable, never a command-line value or `config.toml`; and
- rejects startup when the token is empty and rejects requests with a missing or incorrect bearer
  token.

The bearer token protects the loopback MCP endpoint; it does not turn a shared host account into a
tenant boundary. Use a dedicated high-entropy value, keep it out of shell history and logs, and
stop the foreground relay when the client no longer needs it.

## Remote computers and app tokens

Remote access starts off on each computer. The daemon authenticates remote devices and checks their
granted scopes on each operation. LAN and SSH routes connect devices directly. Desktop and CLI
connections over a hosted route carry end-to-end encrypted traffic through Cloudflare. Browser
connections over a hosted route terminate browser TLS at Cloudflare and use device-bound credentials.

App tokens work on the local HTTP listener. For a browser integration, configure exact origins on
that token. Webhook endpoints authenticate deliveries with the trigger's signing secret and require
confirmation when their advertised address changes. [Remote Computers](https://compozy.com/docs/computers/remote-host)

Compozy Cloud stores account, device, plan, and route metadata. Sessions, source files, prompts, and
transcripts stay on the computer running the daemon. Cloud admin access is restricted to enrolled
operators. Account suspension revokes active credentials, invalidates hosted access, and records an
audit event.

Before reopening traffic after a Cloud database restore, the operator advances every account’s
security epoch, reapplies revocations from the external security log, and invalidates restored web
sessions and device sessions or token instances issued after the restore point. The restore runbook
also reconciles current Stripe entitlements and Cloudflare tunnels.

## Secret redaction

CompozyOS always applies exact protection for claim tokens, secret references, and secrets registered by
the Vault or another runtime subsystem. `redact.enabled` controls an additional heuristic for likely
credentials in free text and log messages. The daemon snapshots this setting at boot, so a change is
restart-required. Disabling it does not disable the exact protections.

When the heuristic is enabled, matching content is redacted before append to the global event ledger
and per-session `events.db`; SSE, history, and resume replay consume the stored redacted form.
Structured correlation values such as session IDs, run IDs, hashes, digests, and fingerprints are
not heuristic candidates.

Heuristic redaction is defense in depth, not an authorization or isolation boundary. It can only
recognize supported credential shapes and entropy patterns. Keep raw secrets out of prompts, tool
results, task descriptions, memory, and other agent-visible content whenever possible.
