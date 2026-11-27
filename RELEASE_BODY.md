## 0.7.9 - 2026-11-27

### Bug Fixes

- Accept workspace_id matcher for subagent hooks (#1950)
- Coerce --input by declared Loop input type
- Show needs-you on collapsed session folders (#1997)
- Pass cloud environment secrets to the staging deploy (#1998)
- Run cloud migrations as a non-superuser role (#1999)
- Copy every cloud dependency into the cloud image (#2000)
- Isolate tool provider failures from unrelated CompozyOS tools (#2003)
- Accept boolean JSON subschemas and tuples in MCP tool schemas (#2001)
- Launch the GitHub MCP server with its stdio subcommand (#2005)
- Request resource-advertised offline_access and accept narrower MCP OAuth grants (#2008)
- Route managed delegation through CompozyOS subagents (#2014)
- Balance daemon integration shards using CI timings
- Give daemon integration shards more headroom
- Accept ACP speed options in the "speed" category (#2028)
- Honor the target agent runtime in subagent delegation (#2038)
- Reopen persisted vault keys on Windows (#2034)
- Report malformed extension manifest errors (#2037)
- Settle child turns watched by their creator (#2035)
- Keep cloud deploy tags out of release version bumps (#2050)
- Let the cloud sign-in form run Turnstile with a deferred api.js (#2047)
- Keep starved task runs with their owner session (#2041)
- Stabilize the session delete confirmation (#2031)
- Honor authored Loop iteration caps (#1949)
- Resume sessions after late clarification answers (#2039)
- Refresh the session MCP catalog after a settings check (#2040)
- Keep Sessions open after deleting a session (#2049)
- Reuse workspace inputs when re-enabling extensions (#2045)
- Show one chevron in the session environment picker (#2053)
- Show only live session work on the dashboard (#2048)

### Features

- Let extensions declare min_compozy_version (#1952)
- Offer spec_conductor delivery when cy-create-spec closes (#1937)
- Allow fallback_chain in CompozyOS agent overrides (#1941)
- Mark the open always-on agent on its row, seat and +N crew (#1938)
- Support structured handoff and prior subagent linkage on delegation
- Resolve loop inputs at fire time
- Authored terminal status for gates and contract
- Put running activity in the sidebar's leading column
- Archive stopped session threads in one action
- Let extension inputs reach the extension process and add setup
- Require evidence-backed lessons on CompozyOS skill proposals
- Let extensions declare and install experimental dependencies
- Add Remote Computers, cloud hosting, and required sign-in (#1994)
- Install CompozyOS skills through a crash-safe transaction (#1966)
- Complete CLI identifier values from the daemon (#2029)
- Show Antigravity (agy) provider usage on macOS (#2042)
- Propose edits to CompozyOS skills and revert approvals (#2044)

### Release Notes

#### Breaking Changes

##### Remote Computers, one-time sign-in, and Compozy Pro

CompozyOS 0.8.0 introduces **Remote Computers**, **Compozy Pro**, and one-time account sign-in for
official release builds, and removes the legacy Remote Gateway.

Read the launch post: [CompozyOS 0.8: your agents on all your computers](https://compozy.com/blog/compozy-0-8-remote-computers).

## Highlights

- **Remote Computers:** add Linux or macOS machines over OpenSSH with verified release installs and
  managed user services (`systemd --user` / `launchd`), pick where each session runs with
  **Run on**, merge sessions across computers in the sidebar, Kanban, and Search, and use remote
  terminals, files, Changes, worktrees, tasks, and provider sign-in.
- **Auto load balancing:** score connected computers with the repository by live free CPU and
  memory, or disable local agent execution on thin controllers.
- **Hosted routes and phone access (Pro):** turn on a hosted route (`compozy.run`) on up to 10
  computers at once with no router or VPN setup, open your computers from a phone browser PWA, and
  route hosted webhooks to your existing trigger paths.
- **Compozy Pro:** Free includes every local feature and 1 remote computer over LAN or SSH; Pro
  adds unlimited remote computers, hosted routes, phone/browser access, and hosted webhooks at
  USD 9.99/month or USD 79.99/year (save 33%, plus applicable taxes calculated at checkout). See
  [compozy.com/pricing](https://compozy.com/pricing).

## Migration notes

### Mandatory sign-in on release builds

Official CompozyOS 0.8.0 release builds ask you to sign in once with Google or GitHub on first run
(`compozy account login` or the desktop app's sign-in screen).

- **Existing installations keep all data:** sessions, workspaces, agents, Loops, settings, and
  scheduled jobs remain intact after updating and signing in.
- **First sign-in needs the internet; later local use works offline.** Temporary cloud outages
  never sign an enrolled installation out.
- **What stays on your computers:** prompts, transcripts, source code, files, and model API keys
  never leave your machines. See [Why CompozyOS asks you to sign in](https://compozy.com/docs/account/sign-in).
- **CI and headless environments:** create a local-only account token with
  `compozy account token create --name ci` and pass it in `COMPOZY_ACCOUNT_TOKEN`. See
  [Account tokens](https://compozy.com/docs/account/tokens).

Sign in on unattended machines before relying on scheduled work. Running turns and claimed leases
finish during sign-out. Recurring jobs skip occurrences until sign-in, and each held one-shot job
runs once when work resumes. Webhooks ask the sender to retry. For SSH provisioning, the controller
can link the host with a setup token before its first sign-in.

### Remote Gateway removed

The legacy Remote Gateway (`internal/gateway`, the bundled Tailscale provider, pairing codes, and
`[gateway]` in `config.toml`) has been removed and replaced by Remote Computers.

| Removed command   | Replacement                                             |
| ----------------- | ------------------------------------------------------- |
| `compozy connect` | `compozy computer`                                      |
| `compozy gateway` | `compozy remote`                                        |
| `compozy pair`    | `compozy remote link` / `compozy account login`         |
| `compozy device`  | `compozy remote devices` / Settings → Account → Devices |

- Removed commands and `--confirm-gateway-requirement` print a migration pointer and exit 2 in 0.8.x.
  The pointers expire in 0.9.0.
- Paired devices, CLI connection profiles and their credentials, and connectivity providers and
  their scaffolds are removed. Add your computers again from **Settings → Computers** or with
  `compozy computer`.
- App tokens (with allowed origins) and webhook triggers are preserved automatically. Webhook
  triggers whose public base address changed show **Needs confirmation**. Confirm them and update the
  sender's URL. Update scripts to use `/api/ingress/bindings` and remove references to other Gateway
  routes and `compozy__gateway`. For public webhook delivery, use a hosted route or set
  `remote.ingress.public_url` for your HTTPS tunnel. [Remote Computers setup](https://compozy.com/docs/computers)
- Former gateway users receive a one-time notice in Settings linking the launch post and a
  migration coupon for **3 months of Compozy Pro free** (redeemable in Checkout for 60 days).

### Official install methods

`go install` is no longer an official install method. Use the desktop app, the verified installer
(`curl -fsSL https://compozy.com/install.sh | sh`), Homebrew, npm, or Linux release packages from
[compozy.com/download](https://compozy.com/download). Building from source is documented in
`.github/CONTRIBUTING.md` and produces a development build without the sign-in gate or
Compozy Cloud features.

#### Fixes

##### Loops honor authored iteration caps

New Loop runs use a positive `contract.iteration_cap` from their definition ahead of the
delivery or watch defaults. A run that needs another generation after reaching its cap
ends as `exhausted`. An omitted or zero contract cap uses the configured fallback.

## Migration notes

Review existing definitions with a positive cap if they relied on a higher delivery limit or
on an unbounded watch run. Set a different cap with
`compozy loop configure --name <name> --set iteration_cap=<n>` or set `n` to `0` to make
the Loop unbounded. Previously admitted runs keep the cap pinned at admission.
