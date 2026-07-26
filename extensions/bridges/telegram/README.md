# Telegram Bridge Provider

`extensions/bridges/telegram` is the production Telegram bridge provider for AGH. It runs as a provider-scoped subprocess on top of `internal/bridgesdk` and multiplexes one or more owned `BridgeInstance` records inside a single Telegram runtime.

It implements:

- provider-scoped Host API ownership through `bridges/instances/list`, `bridges/instances/get`, `bridges/instances/report_state`, and `bridges/messages/ingest`
- hardened webhook ingress with method/content-type/body-size/rate-limit/in-flight checks plus Telegram secret-token verification
- direct-chat and group/forum routing identity mapping into bridge v1 inbound envelopes
- outbound `sendMessage`, `editMessageText`, and `deleteMessage` behavior for bridge delivery requests
- resume handling for the remote message recorded by the shared bridge delivery broker

## Build and install

Released `agh` artifacts do not include this provider executable. From a trusted AGH source
checkout, run this from the repository root with the daemon running:

```bash
mkdir -p ./extensions/bridges/telegram/bin
go build -o ./extensions/bridges/telegram/bin/telegram ./extensions/bridges/telegram
agh extension install ./extensions/bridges/telegram --allow-unverified --yes -o json
agh extension status telegram -o json
```

## Provider Config

The bridge instance `provider_config` JSON object currently supports:

```json
{
  "webhook": {
    "public_url": "https://bridge.example.com/telegram/support",
    "listen_addr": "127.0.0.1:8080",
    "path": "/telegram/brg-main"
  },
  "dm": {
    "allow_user_ids": ["12345"],
    "allow_usernames": ["alice"],
    "paired_user_ids": ["12345"],
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

- `bot_token` is required through bridge secret bindings.
- `webhook_secret` is optional; when set, inbound requests must include `X-Telegram-Bot-Api-Secret-Token`.
- `AGH_BRIDGE_TELEGRAM_LISTEN_ADDR` configures the process-level listener default.
- `AGH_BRIDGE_TELEGRAM_API_BASE_URL` is an operator-owned process override for local development and integration tests. Bridge config cannot change the credential-bearing API destination.
- Direct-message enforcement uses the bridge instance `dm_policy` plus the provider-config allowlist or paired-user fields.

## Outbound text

The provider converts common Markdown constructs to Telegram MarkdownV2 before measuring the 4,096 UTF-16 code-unit wire limit. Long text is split on natural boundaries with numbered, fence-balanced continuations in the original topic. Streaming previews keep one mutable message; terminal delivery posts the complete continuation set and acknowledges its last message. A MarkdownV2 parse rejection retries that chunk once as plain text without `parse_mode`.

## Tool progress

Telegram instances default to `tool_progress: new` with accumulated updates, typing actions, and reactions enabled. The provider posts one progress bubble in the inbound topic, schedules edits on the shared 1.5-second interval, and sends the answer as a separate message in that topic. Progress text uses the same MarkdownV2 formatter, plain-text fallback, and 4,096 UTF-16 code-unit limit as regular delivery.

Progress uses `sendMessage`, `editMessageText`, `sendChatAction`, and `setMessageReaction`. Reactions apply to the progress bubble. Sending the answer clears Telegram's typing indicator; Telegram has no explicit clear-typing method. Rate-limited edits honor the Bot API `retry_after` value before retrying. Set `delivery_defaults.progress.tool_progress` to `off` to acknowledge progress without Telegram API calls.

See the [Telegram operator setup guide](../../../packages/site/content/runtime/core/bridges/setup-telegram.mdx)
for BotFather setup, webhook registration, route selection, verification, delivery testing, and
troubleshooting.
