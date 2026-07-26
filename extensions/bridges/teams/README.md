# Teams Bridge Provider

Production Microsoft Teams bridge provider built on `internal/bridgesdk`.

## Build and install

Released `agh` artifacts do not include this provider executable. From a trusted AGH source
checkout, run this from the repository root with the daemon running:

```bash
mkdir -p ./extensions/bridges/teams/bin
go build -o ./extensions/bridges/teams/bin/teams ./extensions/bridges/teams
agh extension install ./extensions/bridges/teams --allow-unverified --yes -o json
agh extension status teams -o json
```

## Secrets

- `app_id`: Microsoft bot application ID.
- `app_password`: Microsoft bot client secret.
- `app_tenant_id`: optional single-tenant pinning for outbound Bot Framework token acquisition and DM creation.

## Provider Config

Provider config is stored per bridge instance in `provider_config`:

```json
{
  "webhook": {
    "public_url": "https://bridge.example.com/teams/support",
    "listen_addr": "127.0.0.1:0",
    "path": "/teams/brg-example"
  },
  "batching": {
    "delay_ms": 50,
    "split_delay_ms": 50,
    "split_threshold": 2
  },
  "dm": {
    "allow_user_ids": ["29:example"],
    "paired_user_ids": ["29:paired"]
  }
}
```

Teams learns the service URL from authenticated inbound activities and uses the Bot Framework default as the proactive-delivery fallback. `AGH_BRIDGE_TEAMS_SERVICE_URL`, `AGH_BRIDGE_TEAMS_OPENID_METADATA_URL`, and `AGH_BRIDGE_TEAMS_TOKEN_URL` are operator-owned process overrides for local development and integration tests. Bridge config cannot change these credential-bearing destinations.

## Scope

Bridge v1 support in this provider includes:

- inbound Teams message activities
- inbound adaptive-card/message-submit actions
- inbound message reactions
- outbound post, edit, and delete delivery
- tenant-aware proactive DM creation when only a user ID is available

Task modules, modal lifecycle flows, and richer Teams UI parity stay out of scope for v1.

## Delivery behavior

Teams accepts up to 28,000 Unicode code points per activity. The provider splits longer terminal replies into ordered activities and adds `(N/M)` on a separate line. It preserves the conversation and reply reference for every continuation.

While an agent response is still streaming, an overflowing preview stays in one editable activity. On the terminal update, the provider edits that activity, posts the remaining chunks as replies, and acknowledges the final activity ID.

## Tool progress

Tool progress is off by default. Enable accumulated progress for a bridge instance through `delivery_defaults`:

```json
{
  "progress": {
    "tool_progress": "all",
    "grouping": "accumulate",
    "typing": true,
    "reactions": false
  }
}
```

The provider posts one markdown activity in the triggering conversation and updates that activity as progress arrives. It sends a Teams `typing` activity before active work. The bridge does not send an explicit typing-clear activity or outbound progress reactions because those operations are not exposed by its Bot Framework API surface.

See the [Microsoft Teams operator setup guide](../../../packages/site/content/runtime/core/bridges/setup-teams.mdx)
for bot/app provisioning, package upload, endpoint verification, route selection, delivery testing,
and troubleshooting.
