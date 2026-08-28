# BUG-20260827-cursor-launch-model-negotiation: Cursor launch aliases are rejected as unavailable models

- **Status:** fixed, pending remediation commit
- **Impact (user-side):** Blocks-Completion
- **Severity:** Critical · **Priority:** P0
- **Persona Affected:** Théo
- **Journey Step:** J-17 Send the first prompt with a launch-bound Cursor runtime
- **Scenarios:** RT-070; RT-cursor-logical-runtime-options
- **Found:** 2026-08-27 · **Report:** docs/qa/reports/2026-08-27-acp-runtime-catalog.md

## Summary

Compozy passed the correct private Cursor alias to the CLI, then rejected the resulting ACP session because it treated Cursor's shared ACP configuration state as proof of the model selected for that process.

## Reproduction

- **Charter:** CH-provider-runtime-strategies · **Tour:** Feature Tour
- **Environment:** desktop / wifi-fast / en-US; isolated local daemon with the operator's native Cursor login

1. Create an agent with Cursor `grok-4.6`, `xhigh`, and Fast.
2. Create a session from the agent.
3. Send the first prompt.

**Expected:** Compozy validates and compiles the logical runtime before launch, passes the private alias through `--model`, and does not renegotiate a launch-bound runtime from shared ACP configuration state.
**Actual:** Negotiation returned `provider.negotiation.model_unavailable` after `session/new` exposed a different shared Cursor selection, even though the launched process had already received the correct alias.

## Evidence

- Failure screenshot: `/Users/pedronauck/dev/qa-labs/compozy-acp-runtime-catalog-20260828-004625-083662-lab/qa-artifacts/qa/evidence/web-session-first-prompt-launch-bind-failure.png`.
- A direct `cursor-agent --model cursor-grok-4.5-high-fast --print` run reported the selected Cursor Grok 4.5 High model, while ACP `session/new` continued to expose Cursor's separate shared configuration selection.

## Fix

- **Root cause:** the launch-bound runtime had already been catalog-validated and compiled into the process command, but post-launch verification compared it with ACP `current_value_id`. Cursor owns that value as shared configuration state rather than an authoritative echo of the process launch argument.
- **Fix:** launch-bound strategies stop after the process and mode handshake; only `session_config` strategies negotiate runtime through ACP configuration methods.
- **Fix commit:** pending; included in the single remediation commit
- **Regression test:** `TestStartDoesNotRenegotiateLaunchBoundRuntimeAgainstSessionConfig` proves a different shared ACP selection cannot veto a prevalidated launch-bound runtime and that no `session/set_config_option` request is sent.

## Verification

- **Retested:** 2026-08-27 in the same isolated lab after rebuilding and restarting the daemon.
- **Result:** pass — a fresh Web session answered `QA_RUNTIME_FAST_OK`; CLI readback reported logical `grok-4.5`, reasoning `high`, speed `fast`, and `initial_bind` ready state.
- **Evidence:** `/Users/pedronauck/dev/qa-labs/compozy-acp-runtime-catalog-20260828-004625-083662-lab/qa-artifacts/qa/evidence/web-session-first-prompt-grok45-fast-pass.png`
