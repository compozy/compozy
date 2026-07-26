# Linear Bridge Provider

`extensions/bridges/linear` connects one Linear organization to AGH through issue comments or Linear
Agent Sessions. Runtime mode and authentication mode are independent and must be configured
explicitly.

## Runtime behavior

- `comments` mode ingests new `Comment` webhooks and delivers editable Linear comments.
- `agent_sessions` mode ingests `AgentSessionEvent` `created`/`prompted` actions and delivers
  append-only Agent Activities.
- `api_key` authentication uses a bound Linear API key.
- `oauth` authentication uses the client-credentials grant.
- Every accepted webhook and live viewer identity must match `organization_id`.
- Tool progress is acknowledged without creating Linear artifacts.

## Build and install

Released `agh` artifacts do not include this provider executable. From a trusted AGH source
checkout, run this from the repository root with the daemon running:

```bash
mkdir -p ./extensions/bridges/linear/bin
go build -o ./extensions/bridges/linear/bin/linear ./extensions/bridges/linear
agh extension install ./extensions/bridges/linear --allow-unverified --yes -o json
agh extension status linear -o json
```

## Secrets

| Slot             | Required     | Meaning                                 |
| ---------------- | ------------ | --------------------------------------- |
| `webhook_secret` | yes          | Linear webhook/app HMAC signing secret. |
| `api_key`        | API-key mode | Integration or dedicated-user API key.  |
| `client_id`      | OAuth mode   | Linear OAuth application client ID.     |
| `client_secret`  | OAuth mode   | Linear OAuth client-credentials secret. |

## Provider config

```json
{
  "organization_id": "org_123",
  "mode": "comments",
  "auth_mode": "api_key",
  "webhook": {
    "public_url": "https://bridge.example.com/linear/product",
    "listen_addr": "127.0.0.1:18089",
    "path": "/linear/product"
  }
}
```

`AGH_BRIDGE_LINEAR_LISTEN_ADDR` supplies the listener fallback.
`AGH_BRIDGE_LINEAR_API_BASE_URL` and `AGH_BRIDGE_LINEAR_TOKEN_URL` are trusted process overrides for
integration tests or sovereign deployments. Bridge config cannot redirect bound credentials.

## Known limits

- Comment mode starts turns only from new comments.
- Agent Activities are append-only; edit and delete delivery are unsupported in Agent Session mode.
- `agh bridge verify` reports provider identity as `skipped`; enabled runtime health performs the live
  GraphQL viewer check.
- Generic outbound media and provider-visible tool progress are not implemented.

See `packages/site/content/runtime/core/bridges/setup-linear.mdx` for the full operator journey.
