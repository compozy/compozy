# GitHub Copilot CLI as an ACP provider — 2026-11-03

Verdict: PASS for `RT-copilot-acp-provider` on a Copilot Free account, except the paid-plan model
and effort picker. That picker was verified only by tests built from a recorded paid-plan shape.

Setup: Copilot CLI 1.0.105 (`npm install -g @github/copilot`), installed under the operator's
`mise` Node. The account is Copilot Free (`free_limited_copilot`): one model, Auto resolves to
`gpt-6-luna`, and there are no premium requests. The lab was an isolated daemon built from this
branch, with its own `COMPOZY_HOME`, HTTP port, and socket. `COPILOT_HOME` was exported to the lab
daemon. Copilot authenticated from the macOS keychain and copied no credentials. The operator's
`~/.copilot` session store and logs kept their pre-run timestamps (before/after `stat` diff empty).
Four short prompts reached Copilot.

## Proven live

- **Readiness.** `provider inspect copilot` reports `login.source=auth_login_command`,
  `executable=copilot`, and `presence=present`. `provider auth status copilot --remote` reports
  `authenticated` after a promptless ACP session. `provider auth login copilot` was not run
  interactively, because the account was already signed in. Its wiring to `copilot login` is
  covered by `TestProviderAuthLoginCommand`.
- **Signed out.** The lab overrode the command with a separate `HOME`, no `gh` config, and no `gh`
  on `PATH`. The remote probe reports `needs_login` with action `login`. A prompt fails with
  `provider_not_authenticated`: "Sign in locally with compozy provider auth login copilot." Before
  the start-classification fix in this branch, the same prompt returned the raw
  `{"code":-32000,"message":"Authentication required"}`.
- **Native tools.** In an `approve-reads` session, Copilot called
  `compozy-hosted-tools-compozy__tool_search`, `__skill_view`, and `__workspace_info` through the
  loopback bridge, and every call completed. The daemon logs the `mcp_bridge` start stage. Neither
  the logs nor the session store contains a bearer value.
- **Permissions.** The shell command raised a pending `permission` interaction with `allow-once`,
  `allow-always`, and `reject-once`. `session approve --decision allow-once` ran it, and
  `probe.txt` was written. A mutating native tool (`compozy__monitor_create`) raised both the
  provider permission and the CompozyOS tool-gateway approval.
- **Models and effort (Free).** `provider_live:copilot` fails with "inspected session has no model
  option" and publishes no rows. `--model gpt-5.4` fails with `model_unavailable`, and
  `--reasoning-effort high` fails with `reasoning_option_missing`. Both fail before any prompt, and
  the session stays healthy.
- **Unrestricted options.** Under `approve-reads`, `--acp-option allow_all=on` and the Autopilot
  mode are refused with "requires approve-all permissions". With `provider_full_access = true` and
  `approve-all`, a new Copilot agent resolves `allow_all=on`. The session then starts, and start
  fails unless Copilot confirms that option.
- **Cancel.** `session prompt-cancel` ended the turn as `cancelled` while it waited on approval. The
  session returned to idle and healthy, and the cancelled command left no side effect.
- **Resume.** After `session stop`, the bridge listener was gone and only the daemon port remained.
  The next prompt reloaded the same Copilot session (`session_load` succeeded on a new bridge port)
  and called `compozy__workspace_info` again. The reply quoted the word written earlier.

- **Bridge over SSE responses (retest).** The bridge later moved from JSON to SSE responses so it
  can relay progress. A fresh isolated lab reran one prompt: Copilot called
  `compozy__skill_view`, `compozy__tool_info`, and `compozy__workspace_info` through the bridge, and
  each completed. The reply named the workspace, the logs held no bearer value, and the operator's
  `~/.copilot` stayed unchanged.

## Not proven live

- **Paid-plan model and effort picker.** Covered by `internal/modelcatalog/live_sources_test.go`
  with the recorded 1.0.88 paid-plan option shape (`model` plus per-model `reasoning_effort`).
- **Interactive `copilot login` through CompozyOS.** Covered by the CLI test only.
- **Web UI.** The Copilot mark is covered by `@compozy/ui` tests and the logo story. No browser walk
  was done.

## Observations

- `session stop` during idle recorded `session.stop_escalated` (`phase: forced`, 7 ms). Lab
  teardown also had to escalate the daemon stop. Neither blocked the walk.
- `TestAgentPreStartHookOpenCodeLaunchOrigin` fails on macOS on `main` too (`/private/var` vs
  `/var`). It is unrelated to this change.

Scratch evidence was in the lab under `qa-artifacts/ev/`. Teardown recorded `"clean": true`.
