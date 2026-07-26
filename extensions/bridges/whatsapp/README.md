# WhatsApp Bridge Provider

`extensions/bridges/whatsapp` is the production WhatsApp Cloud API bridge provider for AGH. It runs as a provider-scoped subprocess on top of `internal/bridgesdk` and multiplexes one or more owned `BridgeInstance` records inside a single WhatsApp runtime.

It implements:

- provider-scoped Host API ownership through `bridges/instances/list`, `bridges/instances/get`, `bridges/instances/report_state`, and `bridges/messages/ingest`
- hardened webhook ingress with verify-challenge GET handling plus signed POST validation through `X-Hub-Signature-256`
- direct-message style inbound mapping for WhatsApp Cloud message webhooks
- outbound text delivery through the Cloud API with shared chunking and retry or rate-limit classification
- resume handling for the remote message recorded by the shared bridge delivery broker

## Outbound delivery

WhatsApp text messages are limited to 4,096 Unicode code points. AGH splits larger replies on natural boundaries, adds an `(N/M)` marker to every chunk, sends the chunks in order, and acknowledges the last `wamid` returned by the Cloud API.

The Cloud API does not expose text-message editing. When an accumulated delivery changes, AGH posts a new chunk sequence and keeps the prior remote message ID as the replacement reference.

## Tool progress

Tool progress is off by default. Enable sparse progress posts for a bridge instance through `delivery_defaults`:

```json
{
  "progress": {
    "tool_progress": "new",
    "grouping": "separate",
    "typing": false,
    "reactions": false
  }
}
```

The provider sends each projected status as a separate text message. The daemon applies `new`-mode tool projection before the provider receives an event. WhatsApp progress does not use edit, typing, or reaction operations. If an instance selects `grouping: "accumulate"`, updates append only the newest status line as another text message because the Cloud API cannot edit an existing text message.

## Build and install

Released `agh` artifacts do not include this provider executable. From a trusted AGH source
checkout, run this from the repository root with the daemon running:

```bash
mkdir -p ./extensions/bridges/whatsapp/bin
go build -o ./extensions/bridges/whatsapp/bin/whatsapp ./extensions/bridges/whatsapp
agh extension install ./extensions/bridges/whatsapp --allow-unverified --yes -o json
agh extension status whatsapp -o json
```

## Provider Config

The bridge instance `provider_config` JSON object currently supports:

```json
{
  "api_version": "v21.0",
  "phone_number_id": "1234567890",
  "webhook": {
    "public_url": "https://bridge.example.com/whatsapp/support",
    "listen_addr": "127.0.0.1:8080",
    "path": "/whatsapp/brg-main"
  },
  "dm": {
    "allow_user_ids": ["15551234567"],
    "allow_usernames": ["alice example"],
    "paired_user_ids": ["15551234567"],
    "paired_usernames": ["alice example"]
  },
  "batching": {
    "delay_ms": 0,
    "split_delay_ms": 0,
    "split_threshold": 0
  }
}
```

Notes:

- `access_token`, `app_secret`, and `verify_token` are required through bridge secret bindings.
- `provider_config.phone_number_id` is required per bridge instance because the runtime multiplexes multiple business numbers behind one provider process.
- `AGH_BRIDGE_WHATSAPP_LISTEN_ADDR` configures the process-level listener default.
- `AGH_BRIDGE_WHATSAPP_API_BASE_URL` is an operator-owned process override for local development and integration tests. Bridge config cannot change the credential-bearing API destination.
- Direct-message enforcement uses the bridge instance `dm_policy` plus the provider-config allowlist or paired-user fields.
- WhatsApp Cloud API does not support bridge-level delete semantics and the provider reports those requests as permanent unsupported operations.

See the [WhatsApp operator setup guide](../../../packages/site/content/runtime/core/bridges/setup-whatsapp.mdx)
for Meta app setup, challenge/signature verification, route access, real delivery testing, and
troubleshooting.
