---
title: Remote Computers, one-time sign-in, and Compozy Pro
type: breaking
---

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
