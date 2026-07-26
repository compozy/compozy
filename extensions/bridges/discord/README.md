# Discord Bridge Provider

`extensions/bridges/discord` is the production Discord bridge provider for AGH. It runs as a provider-scoped subprocess on top of `internal/bridgesdk` and multiplexes one or more owned `BridgeInstance` records inside a single Discord runtime.

It implements:

- provider-scoped Host API ownership through `bridges/instances/list`, `bridges/instances/get`, `bridges/instances/report_state`, and `bridges/messages/ingest`
- hardened webhook ingress with method/content-type/body-size/rate-limit/in-flight checks plus Discord Ed25519 signature verification
- Discord `MESSAGE_CREATE`, reaction add/remove, application command, and component events mapped to typed bridge flows
- outbound Discord REST create, edit, and delete behavior for bridge delivery requests
- live tool progress through Discord message create/edit, typing, and reaction endpoints
- resume handling for the remote message recorded by the shared bridge delivery broker

The webhook endpoint verifies `X-Signature-Timestamp` and `X-Signature-Ed25519` over the exact
request body with the bound 32-byte Ed25519 public key. Discord's signed interaction PING receives a
PONG response. Ordinary `MESSAGE_UPDATE` events are not ingested because this provider implements
HTTP interactions and Application Webhook Events, not a Discord Gateway lifecycle.

## Delivery behavior

Discord accepts up to 2,000 Unicode code points per message. The provider splits longer terminal replies into ordered messages and adds `(N/M)` on a separate line so operators can follow the sequence. Every message remains within Discord's limit.

While an agent response is still streaming, an overflowing preview stays in one editable message. On the terminal update, the provider edits that active message, posts the remaining chunks as continuations, and acknowledges the final message ID.

Tool progress defaults to `new` with `accumulate` grouping. The provider posts the first progress line to the resolved Discord destination, edits that same message as more lines arrive, and keeps the progress message ID separate from the final-answer acknowledgement. Started, completed, and failed tool phases use phase-specific processing, success, and failure reactions on the progress message itself. Edits use the shared throttle and honor Discord `Retry-After` responses.

Discord exposes a start-typing endpoint without a matching clear request. The provider stops issuing typing requests when text is delivered and closes progress state at terminal delivery. Set `delivery_defaults.progress.tool_progress` to `off` to acknowledge progress without Discord API calls.

## Build and install

Released `agh` artifacts do not include this provider executable. From a trusted AGH source
checkout, run this from the repository root with the daemon running:

```bash
mkdir -p ./extensions/bridges/discord/bin
go build -o ./extensions/bridges/discord/bin/discord ./extensions/bridges/discord
agh extension install ./extensions/bridges/discord --allow-unverified --yes -o json
agh extension status discord -o json
```

## Provider Config

The bridge instance `provider_config` JSON object currently supports:

```json
{
  "application_id": "111122223333444455",
  "webhook": {
    "public_url": "https://bridge.example.com/discord/support",
    "listen_addr": "127.0.0.1:8080",
    "path": "/discord/brg-main"
  },
  "invite": {
    "scopes": ["bot", "applications.commands"],
    "permissions": 68672
  },
  "dm": {
    "allow_user_ids": ["123456789012345678"],
    "allow_usernames": ["alice"],
    "paired_user_ids": ["123456789012345678"],
    "paired_usernames": ["alice"]
  },
  "batching": {
    "delay_ms": 0,
    "split_delay_ms": 0,
    "split_threshold": 0
  }
}
```

Notes:

- `bot_token` and the Ed25519 `public_key` are required through bridge secret bindings.
- `application_id` is checked against the authenticated bot identity when configured.
- `webhook.public_url`, `invite.scopes`, and `invite.permissions` are daemon setup and verification metadata. The adapter listens on `webhook.listen_addr` plus `webhook.path`; the setup wizard uses the public URL and invite values for the operator handoff.
- `AGH_BRIDGE_DISCORD_LISTEN_ADDR` configures the process-level listener default.
- `AGH_BRIDGE_DISCORD_API_BASE_URL` is an operator-owned process override for local development and integration tests. Bridge config cannot change the credential-bearing API destination.
- Direct-message enforcement uses the bridge instance `dm_policy` plus the provider-config allowlist or paired-user fields.

See the [Discord operator setup guide](../../../packages/site/content/runtime/core/bridges/setup-discord.mdx)
for application setup, route selection, endpoint verification, real inbound proof, delivery testing,
and troubleshooting.
