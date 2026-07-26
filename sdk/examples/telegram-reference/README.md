# Telegram Reference Conformance Runtime

`telegram-reference` is the provider-scoped bridge conformance runtime for AGH. It is not the production Telegram provider. Its job is to exercise the shared `internal/bridgesdk` runtime, Host API surface, and reusable harness contract that future provider binaries must satisfy.

It demonstrates:

- launch-time provider metadata and managed bridge instance grants through `initialize.runtime.bridge`
- owned-instance Host API access through `bridges/instances/list` and explicit `bridges/instances/get`
- inbound platform normalization through `bridges/messages/ingest`
- outbound negotiated delivery through `bridges/deliver`
- adapter-driven per-instance state reporting through `bridges/instances/report_state`
- restart-safe delivery markers that the provider-scoped conformance harness can validate

This example is intentionally fake-platform and CI-safe. Instead of talking to the real Telegram API, it tails a JSONL file of Telegram-like updates and writes JSON/JSONL markers that the integration harness reads back.

## Build

From the repository root:

```bash
go build -o ./sdk/examples/telegram-reference/bin/telegram-reference ./sdk/examples/telegram-reference
```

Or from this directory:

```bash
mkdir -p bin
go build -o ./bin/telegram-reference .
```

## Install

Build the binary first, then install the extension directory:

```bash
agh extension install ./sdk/examples/telegram-reference
```

## Manifest Summary

- Capability: `bridge.adapter`
- Host API actions: `bridges/instances/list`, `bridges/messages/ingest`, `bridges/instances/get`, `bridges/instances/report_state`
- Security grants: `bridge.read`, `bridge.write`
- Extension service: `bridges/deliver`

## Fake Platform Contract

The runtime watches the file named by `AGH_BRIDGE_ADAPTER_UPDATES_PATH`. Each non-empty line must be one Telegram-like update JSON object. The minimal supported shape is:

```json
{
  "update_id": 1001,
  "message": {
    "message_id": 9,
    "date": 1775866800,
    "chat": { "id": 42, "type": "private" },
    "from": { "id": 7, "username": "alice", "first_name": "Alice" },
    "text": "hello"
  }
}
```

## Conformance Markers

The adapter reads these optional environment variables. They are used by the conformance harness and can also help extension authors debug runtime behavior:

- `AGH_BRIDGE_ADAPTER_HANDSHAKE_PATH`: writes the initialize request/response marker as JSON.
- `AGH_BRIDGE_ADAPTER_OWNERSHIP_PATH`: writes the provider-owned `bridges/instances/list` result plus explicit `bridges/instances/get` fetches as JSON.
- `AGH_BRIDGE_ADAPTER_STATE_PATH`: appends one JSON line per reported bridge status.
- `AGH_BRIDGE_ADAPTER_DELIVERY_PATH`: appends one JSON line per `bridges/deliver` request, including the returned ack when available.
- `AGH_BRIDGE_ADAPTER_INGEST_PATH`: appends one JSON line per fake inbound update ingest attempt.
- `AGH_BRIDGE_ADAPTER_UPDATES_PATH`: JSONL file polled for fake inbound Telegram updates.
- `AGH_BRIDGE_ADAPTER_STARTS_PATH`: appends one line per runtime process start.
- `AGH_BRIDGE_ADAPTER_SHUTDOWN_PATH`: appends one line when the daemon sends `shutdown`.
- `AGH_BRIDGE_ADAPTER_CRASH_ONCE_PATH`: if set and the file does not exist yet, the runtime exits on its first outbound delivery after writing the request marker. The broker should then resume delivery after restart.

When the provider runtime owns multiple bridge instances, fake inbound updates should include `bridge_instance_id` so the runtime can route them against the correct owned instance explicitly.

## Bound Credentials

The adapter reads only `initialize.runtime.bridge.bound_secrets`. For the ready path, it expects a `bot_token` binding. If that binding is missing, it reports `auth_required` and never attempts any arbitrary runtime secret lookup.
