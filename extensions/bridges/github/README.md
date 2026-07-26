# GitHub Bridge Provider

`extensions/bridges/github` connects one GitHub repository per bridge instance to AGH. It accepts
signed issue-comment and pull-request review-comment webhooks and delivers replies through GitHub
REST.

## Runtime behavior

- PAT or GitHub App authentication.
- One configured `owner/repository` per bridge instance.
- `issue_comment` and `pull_request_review_comment` events with action `created` start turns.
- Issue/review comments support create, edit, and delete delivery.
- Tool progress is acknowledged without creating GitHub comments or reactions.
- Bot-authored comments can be suppressed through `bot_login`.

## Build and install

Released `agh` artifacts do not include this provider executable. From a trusted AGH source
checkout, run this from the repository root with the daemon running:

```bash
mkdir -p ./extensions/bridges/github/bin
go build -o ./extensions/bridges/github/bin/github ./extensions/bridges/github
agh extension install ./extensions/bridges/github --allow-unverified --yes -o json
agh extension status github -o json
```

## Secrets

| Slot             | Required | Meaning                                  |
| ---------------- | -------- | ---------------------------------------- |
| `webhook_secret` | yes      | HMAC secret for `X-Hub-Signature-256`.   |
| `token`          | PAT mode | Fine-grained repository token.           |
| `app_id`         | App mode | Numeric GitHub App ID.                   |
| `private_key`    | App mode | RSA private key in PKCS#1 or PKCS#8 PEM. |

## Provider config

```json
{
  "mode": "pat",
  "repository": { "full_name": "acme/app" },
  "bot_login": "agh-bot",
  "webhook": {
    "public_url": "https://bridge.example.com/github/acme-app",
    "listen_addr": "127.0.0.1:18088",
    "path": "/github/acme-app"
  }
}
```

App mode also accepts `installation_id`. The provider can learn the installation from accepted app
webhook metadata, but outbound App delivery is incomplete until an installation is available.

`AGH_BRIDGE_GITHUB_LISTEN_ADDR` supplies the listener fallback.
`AGH_BRIDGE_GITHUB_API_BASE_URL` is a trusted process override for integration tests or sovereign
deployments. Bridge config cannot redirect bound credentials.

## Known limits

- One bridge instance owns one repository.
- Only newly created issue and review comments start turns.
- `agh bridge verify` reports provider identity as `skipped`; enabled runtime health performs the live
  PAT/App authentication probe.
- Generic outbound media and provider-visible tool progress are not implemented.

See `packages/site/content/runtime/core/bridges/setup-github.mdx` for the full operator journey.
