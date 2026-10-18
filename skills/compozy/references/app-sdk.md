# App SDK and app tokens

Use this reference when building an external application or administering its access. App SDK
clients call a running daemon; extension SDKs run inside daemon-managed extension subprocesses.
Do not interchange their packages or lifecycle assumptions.

## Choose the client

- TypeScript: install the exact daemon-matching version of `@compozy/client`; call
  `await Compozy.connect()`. Resource methods retain operation IDs, for example
  `compozy.tasks.listTasks({ query: { status: "in_progress" } })`. Arguments use wire field names in
  `path`, `query`, and `body`; methods return the primary 2xx body and throw `CompozyApiError` on
  daemon errors. Import generated types from the public package, not its internal files.
- Go: use `github.com/compozy/compozy/sdk/go/client`, call `client.New(ctx)`, and invoke generated
  methods such as `c.ListTasks(ctx, client.ListTasksParams{})`. The package is standard-library-only
  and shares the `sdk/go/v<version>` release tag.
- Node-only lifecycle: `Compozy.start({ home })` from `@compozy/client/node` connects to an existing
  daemon or starts one. `close({ stopDaemon: true })` stops only the same daemon this client started;
  an attached or replacement daemon must remain running. Use an isolated home for labs.

Node/Bun/Deno discovery uses explicit `baseUrl`, `COMPOZY_URL`, the selected home's `daemon.json`,
then loopback port 2123. Go prefers the selected home's Unix socket before its TCP record. Browser
clients default to `http://localhost:2123`. Use explicit addresses from the current lab manifest in
isolated QA; defaults are not a substitute for the manifest. A major/minor mismatch warns without
blocking the connection; compare the actual connected version before diagnosing missing fields.

Go methods for HTTP-only operations require `client.WithBaseURL` with the daemon's HTTP address:
`MintGatewayStreamTicket`, `CompleteMCPAuthCallback`, `ListOpenAIModels`, `DeliverGlobalWebhook`, and
`DeliverWorkspaceWebhook`. Their routes are absent on UDS, so a socket client gets 404. Read
`x-compozy-transports` in OpenAPI; transport selection does not change authorization.

## Prompts, events, and cancellation

Resolve the workspace with `workspaces.resolveWorkspace({ body: { path } })`, create a session with
`sessions.createSession({ body: { workspace: workspace.id, agent_name } })`, and submit through
`compozy.prompt({ workspaceId, sessionId, message })`. Prompt is a handwritten entry point because
it can return a stream or an accepted result. Branch on `reply.kind`: `stream` exposes `chunks`;
`accepted` exposes `result.status`, `result.delivery`, and `result.queue_position`. In Go,
`c.Prompt(ctx, client.PromptInput{...})` returns a stream with `Accepted()` and `Next()`.

Use `promptResponse()` in a server route to preserve the daemon's UI message stream. Accepted work
returns JSON with status 202, not stream chunks; errors preserve the daemon status and body. The
server owns application user authentication and session authorization.

Read other SSE operations through `compozy.events(operationId, { path, query, signal })` or generated
Go stream methods. Event frames retain event name, event ID, and decoded payload. Persist the last
consumed sequence for reattach. TypeScript event streams reconnect network failures and 5xx with
`Last-Event-ID` using bounded backoff; 4xx ends them. Prompt streams never reconnect or resubmit.

TypeScript `events()` also reconnects after clean server EOF. Abort its signal when monitoring is
finished, including after `session_stopped`; that event alone does not stop resubscription. Go streams
return `io.EOF` on server closure instead.

Disconnecting, aborting a reader, or closing a browser page does not cancel an admitted agent turn.
Recover through the durable transcript before retrying work; reuse an explicit idempotency key for
the same submission. Cancel the turn explicitly through `cancelSessionPrompt` or the existing
prompt-cancel CLI/native surface when cancellation is the operator's intent.

## Administer app tokens locally

There are no app-token native tools. Use structured local CLI or HTTP/UDS management:

```bash
compozy app-token create "Dashboard" --origin http://localhost:3000 -o json
compozy app-token list --state all -o json
compozy app-token update apt_example --origin https://app.example.com -o json
compozy app-token revoke apt_example -o json
```

Creation returns `app_token` metadata and `token` once. Never put the secret in task results, logs,
source files, or retained evidence; redact it. List returns `app_tokens` with `id`, `name`, `origins`,
`created_at`, `last_used_at`, and `revoked_at`. Update returns `app_token`; revoke also returns
`changed` and the stream closure count `canceled`. Confirm mutations through a structured list.

`--origin` on update replaces the complete origin set. `--clear-origins` removes all browser origins
and conflicts with `--origin`. Names contain 1–64 characters, each token allows at most 16 exact
HTTP(S) origins, and at most 64 active tokens exist. Paths, queries, wildcards, and opaque `null`
origins are invalid. `list` defaults to active; valid `--state` values are active, revoked, and all.
Revoking twice succeeds with `changed: false`; updating a revoked token returns 409
`app_token_revoked`; an unknown ID returns `app_token_not_found` (CLI exit 69). Invalid CLI flags
fail before the request with exit 2.

The matching local routes are `POST|GET /api/app-tokens` and
`PATCH|DELETE /api/app-tokens/{app_token_id}`. No new configuration keys or hooks are introduced.
App tokens are distinct from paired devices and must not appear in `compozy device list`.

## Browser boundary

A foreign browser origin needs a bearer token whose own allowlist includes that exact origin.
Successful preflight is not authentication. Only registry-marked app-access operations are allowed;
Terminal, filesystem, settings, Vault, extensions, tool invocation, Gateway, agent kernel, and token
management remain forbidden. See the generated operation table at
https://compozy.com/docs/apps/browser. `Origin: null` is always denied. A prompt-capable app can ask
agents to act with their session permissions: the origin allowlist does not sandbox the allowed app.

Revocation rejects authentication with `app_token_revoked` and closes its open streams within five
seconds. A foreign browser can read that 401 only while another active token still allows its origin.
If revocation removes the last grant, preflight is denied and responses lack the CORS grant; browser
JavaScript sees a CORS/network failure. `events()` then follows its existing reconnect budget and can
end with `CompozyStreamError` after 20 consecutive failures, rather than a readable 401. Abort the
reader when the app should stop waiting. Requests without `Origin` remain tokenless locally; if a
token is supplied, it is validated.
Gateway listeners reject app tokens with `gateway_app_token_local_only` and do not update last use.

## Contract and stability

`GET /api/openapi.json` serves the running release's OpenAPI 3.0.3 document over local HTTP and UDS,
with ETag and gzip. It is not exposed on Gateway listeners. API reference samples and SDK methods
come from the same generated document.

Experimental in 0.4: names and shapes may change in 0.5 without a deprecation window. Pin the exact
version. This applies to the SDK facade/resource layout, Node subpath, Go client, app-token CLI/HTTP,
and browser-access error codes. Runtime OpenAPI and generated operation types retain the stability
of the daemon routes they describe. Tutorials: https://compozy.com/docs/apps.
