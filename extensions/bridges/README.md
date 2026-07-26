# Bridge Providers

Each provider implementation lives in its own directory with an `extension.toml`; there is no
central source-tree registry. Released `agh` artifacts do not include these provider executables or
install them automatically.

## Start here

- Operator setup: `packages/site/content/runtime/core/bridges/setup.mdx`
- In-tree provider author walkthrough: `packages/site/content/runtime/core/bridges/adding-a-bridge.mdx`
- In-repo review checklist: `internal/bridges/ADDING_A_BRIDGE.md`
- CI-safe protocol example: `sdk/examples/telegram-reference`

## Build and install

From a trusted AGH source checkout, run this from the repository root with the daemon running:

```bash
PROVIDER=slack
# Choose: slack, telegram, discord, whatsapp, teams, gchat, github, or linear.

mkdir -p "./extensions/bridges/$PROVIDER/bin"
go build \
  -o "./extensions/bridges/$PROVIDER/bin/$PROVIDER" \
  "./extensions/bridges/$PROVIDER"

agh extension install \
  "./extensions/bridges/$PROVIDER" \
  --allow-unverified \
  --yes \
  -o json

agh extension status "$PROVIDER" -o json
```

The local install is an explicit trust decision. It copies the provider into the AGH extension home
and enables it; use `agh extension enable "$PROVIDER" -o json` only if it was disabled later.

## In-tree providers

| Directory  | Platform        | Inbound model                                 | Outbound model                           |
| ---------- | --------------- | --------------------------------------------- | ---------------------------------------- |
| `slack`    | Slack           | Signed Events API, commands, and interactions | Message create/edit/delete               |
| `telegram` | Telegram        | Secret-token Bot API webhooks                 | Message create/edit/delete               |
| `discord`  | Discord         | Ed25519 interactions and webhook events       | REST message create/edit/delete          |
| `whatsapp` | WhatsApp        | Meta verification challenge and signed POST   | Cloud API text create                    |
| `teams`    | Microsoft Teams | Bot Framework activities and bearer JWTs      | Activity create/edit/delete              |
| `gchat`    | Google Chat     | Direct, Pub/Sub, or hybrid Google JWTs        | Chat message create/edit/delete          |
| `github`   | GitHub          | Signed issue and review comment webhooks      | Issue/review comment create/edit/delete  |
| `linear`   | Linear          | Signed comment or Agent Session webhooks      | Comments or append-only Agent Activities |

Provider-specific build, configuration, limits, and unsupported operations live in each directory's
README. Public console steps and operator recovery live in the site setup and operations guides.
