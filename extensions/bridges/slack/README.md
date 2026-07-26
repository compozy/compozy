# Slack Bridge Provider

`extensions/bridges/slack` is the production Slack bridge provider for AGH. It runs as a provider-scoped subprocess on top of `internal/bridgesdk` and multiplexes one or more owned `BridgeInstance` records inside a single Slack runtime.

It implements:

- provider-scoped Host API ownership through `bridges/instances/list`, `bridges/instances/get`, `bridges/instances/report_state`, and `bridges/messages/ingest`
- hardened webhook ingress with method/content-type/body-size/rate-limit/in-flight checks plus Slack signing-secret verification
- Slack Events API messages plus typed bridge `command`, `action`, and `reaction` ingest flows
- outbound `chat.postMessage`, `chat.update`, and `chat.delete` behavior for bridge delivery requests
- resume handling for the remote message recorded by the shared bridge delivery broker

## Build and install

Released `agh` artifacts do not include this provider executable. From a trusted AGH source
checkout, run this from the repository root with the daemon running:

```bash
mkdir -p ./extensions/bridges/slack/bin
go build -o ./extensions/bridges/slack/bin/slack ./extensions/bridges/slack
agh extension install ./extensions/bridges/slack --allow-unverified --yes -o json
agh extension status slack -o json
```

## Provider Config

The bridge instance `provider_config` JSON object currently supports:

```json
{
  "webhook": {
    "public_url": "https://bridge.example.com/slack/support",
    "listen_addr": "127.0.0.1:8080",
    "path": "/slack/brg-main"
  },
  "dm": {
    "allow_user_ids": ["U12345"],
    "allow_usernames": ["alice"],
    "paired_user_ids": ["U12345"],
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

- `bot_token` and `signing_secret` are required through bridge secret bindings.
- `AGH_BRIDGE_SLACK_LISTEN_ADDR` configures the process-level listener default.
- `AGH_BRIDGE_SLACK_API_BASE_URL` is an operator-owned process override for local development and integration tests. Bridge config cannot change the credential-bearing API destination.
- Direct-message enforcement uses the bridge instance `dm_policy` plus the provider-config allowlist or paired-user fields.

## Outbound text

The provider converts common Markdown constructs to Slack mrkdwn before measuring the wire payload. Text over 40,000 UTF-16 code units is split on natural boundaries with numbered, fence-balanced continuations in the original thread. Streaming previews keep one mutable message; the terminal delivery posts the complete continuation set and acknowledges its last message.

## Tool progress

Slack instances default to `tool_progress: new` with accumulated updates, typing status, and reactions enabled. The provider posts one progress bubble under the inbound `thread_ts`, schedules edits on the shared 1.5-second interval, and posts the answer separately in the same thread. Progress text uses the same mrkdwn formatter and 40,000-UTF-16-code-unit limit as regular delivery.

Progress uses `chat.postMessage`, `chat.update`, `assistant.threads.setStatus`, and `reactions.add`. Reactions apply to the progress bubble. Rate-limited edits honor Slack's `Retry-After` response before retrying. Set `delivery_defaults.progress.tool_progress` to `off` to acknowledge progress without Slack API calls. `chat.delete` is available for explicit delivery deletes; automatic progress cleanup is not enabled.

See the [Slack operator setup guide](../../../packages/site/content/runtime/core/bridges/setup-slack.mdx)
for app creation, manifest handoff, verification, real inbound proof, delivery testing, and
troubleshooting.
