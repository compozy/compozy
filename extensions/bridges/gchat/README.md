# Google Chat Bridge Provider

`extensions/bridges/gchat` connects AGH bridge instances to Google Chat through the Chat REST API. One provider subprocess can own multiple bridge instances with independent ingress modes, routing, credentials, and direct-message policies.

It implements:

- direct webhook, Pub/Sub, or hybrid ingress with Google JWT verification
- message, action, and reaction mapping into typed bridge envelopes
- threaded outbound create, edit, and delete operations
- broker-recorded remote-message state and provider API client reuse

## Outbound delivery

Google Chat accepts messages up to 32,000 UTF-8 bytes. AGH measures the final text payload in bytes and splits larger replies on natural boundaries. Every chunk includes an `(N/M)` marker and stays in the original space and thread.

A non-terminal reply that grows past the limit keeps one bounded preview message. On the terminal update, AGH edits that message with the first chunk, posts the remaining chunks in order, and acknowledges the last Google Chat message name.

## Tool progress

Tool progress is off by default. Enable accumulated progress for a bridge instance through `delivery_defaults`:

```json
{
  "progress": {
    "tool_progress": "all",
    "grouping": "accumulate",
    "typing": false,
    "reactions": false
  }
}
```

The provider creates one plain-text progress message in the triggering space and thread, then patches that message as progress arrives. Progress does not use CardV2, typing, or reaction affordances.

## Build and install

Released `agh` artifacts do not include this provider executable. From a trusted AGH source
checkout, run this from the repository root with the daemon running:

```bash
mkdir -p ./extensions/bridges/gchat/bin
go build -o ./extensions/bridges/gchat/bin/gchat ./extensions/bridges/gchat
agh extension install ./extensions/bridges/gchat --allow-unverified --yes -o json
agh extension status gchat -o json
```

## Configuration

Bind `credentials_json` to Google service-account credentials. `project_number` is optional and enables direct-webhook audience verification. Provider config selects `direct`, `pubsub`, or `hybrid` ingress and can configure webhook, certificate, batching, and direct-message policy settings.

`AGH_BRIDGE_GCHAT_LISTEN_ADDR`, `AGH_BRIDGE_GCHAT_DIRECT_CERTS_URL`, and `AGH_BRIDGE_GCHAT_PUBSUB_CERTS_URL` provide process-level listener and certificate overrides. `AGH_BRIDGE_GCHAT_API_BASE_URL` and `AGH_BRIDGE_GCHAT_TOKEN_URL` are operator-owned process overrides for credential-bearing destinations. Bridge config and `credentials_json.token_uri` cannot change those destinations.

See the [Google Chat operator setup guide](../../../packages/site/content/runtime/core/bridges/setup-gchat.mdx)
for Cloud app setup, direct/Pub/Sub verification, route selection, real delivery testing, and
troubleshooting.
