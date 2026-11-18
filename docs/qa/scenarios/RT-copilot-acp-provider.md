---
id: RT-copilot-acp-provider
area: RT
title: GitHub Copilot CLI sign-in, native tools, permissions, cancel, and resume
persona: Ada
journey: J-15
expected: With the official Copilot CLI on PATH and no [providers.copilot] block, login runs `copilot login`, the remote probe reports readiness, and a session runs a CompozyOS native tool through the loopback MCP bridge, raises Copilot permission requests as CompozyOS interactions, cancels cleanly, and resumes with its context. Models and effort follow what the account advertises; an account without a model option fails explicit model or effort requests before the first prompt. Unrestricted Copilot options require approve-all.
entry_points: compozy provider auth login copilot; compozy provider auth status copilot --remote; compozy provider models refresh copilot; compozy session new; compozy session prompt; compozy session approve; compozy session prompt-cancel; compozy session stop
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: internal/mcpbridge/bridge_test.go;internal/acp/client_start_contract_test.go;internal/modelcatalog/live_sources_test.go;internal/config/provider_test.go;internal/cli/provider_test.go;docs/qa/reports/2026-11-03-copilot-acp-provider.md
last_report: docs/qa/reports/2026-11-03-copilot-acp-provider.md
overlaps: RT-provider-full-access-mode; MS-web-provider-auth-gate; RT-agy-acp-provider
---

story: As an operator whose team uses only GitHub Copilot, I sign in once, start sessions on Copilot, and use CompozyOS tools and approvals the same way I do with other providers.

## Sign in and readiness

1. Install the official CLI (`npm install -g @github/copilot` or `brew install copilot-cli`) and confirm `copilot --version`. Confirm no `[providers.copilot]` block exists.
2. Run `compozy provider inspect copilot -o json`: `login.source` is `auth_login_command`, `login.executable` is `copilot`, and `presence` is `present`. No command text or token appears.
3. Run `compozy provider auth login copilot -o json`: it runs `copilot login` (browser or device code). Skip when the account is already signed in.
4. Run `compozy provider auth status copilot --remote -o json`: `state` is `authenticated` after a promptless ACP session.
5. With Copilot signed out (separate `HOME`, no `gh` login), session start fails with an authentication-required diagnostic whose recovery is the login above.

## Session with native tools and permissions

1. Create an agent with `provider: copilot` and `permissions: approve-reads`, then `compozy session new --agent <agent>`.
2. Prompt it to call `compozy__workspace_info` and run a shell command that writes a file.
3. The native tool call appears as `compozy-hosted-tools-compozy__workspace_info` and completes: the bridge delivered the hosted MCP server over loopback HTTP.
4. The shell command raises a pending `permission` interaction with `allow-once`, `allow-always`, and `reject-once`. `compozy session approve --decision allow-once` lets it run and the file appears.

## Models and effort

1. Run `compozy provider models refresh copilot -o json`. On a plan that advertises a `model` option, `provider_live:copilot` publishes the advertised IDs and per-model `reasoning_efforts`. On a single-model plan (Copilot Free) the source fails with `inspected session has no model option` and publishes no rows.
2. On a single-model plan, `compozy session prompt <id> 'say ok' --provider copilot --model <id>` fails with `model_unavailable`, and `--reasoning-effort high` fails with `reasoning_option_missing`, both before any prompt reaches Copilot. The session stays healthy.
3. On a paid plan, select an advertised model and effort and confirm the acknowledged `model` and `reasoning_effort` options match.

## Unrestricted options

1. Under `approve-reads`, `--acp-option allow_all=on` and `--acp-option mode=https://agentclientprotocol.com/protocol/session-modes#autopilot` are refused with `requires approve-all permissions`.
2. With `[permissions] mode = "approve-all"` alone, a new Copilot agent resolves the `allow_all = on` option; under `auto` it does not.

## Cancel and resume

1. Prompt a long-running shell command, then run `compozy session prompt-cancel <id>`: the turn ends `cancelled`, the session returns to idle and healthy, and the command's side effect never happens.
2. Stop the session, then prompt it again: CompozyOS reloads the Copilot session, the reply uses earlier context, and a native tool call still succeeds through a new bridge.
3. After stop, the previous bridge endpoint no longer accepts connections.

## Isolation

Copilot state for the lab lives under the `COPILOT_HOME` exported to the lab daemon. The operator's `~/.copilot` session store and logs keep their pre-run timestamps.
