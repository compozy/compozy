# QA Run Report — 2026-10-29 — Pi native tools

- Scope: issue #1020, native discovery, execution, monitor wake, recovery, and unavailable-bridge guidance.
- Build: branch `fix/issue-1020-pi-native-tools`; implementation through `58ff579be`.
- Environment: isolated Compozy home/socket, HTTP port 58004; Pi 1.4.0 and pi-acp 0.0.31. Local Ollama/Qwen3 1.7B and 4B on a separate port/model store; no paid inference credentials.
- Verdict: targeted real integration passed; unassisted small-model persona failed. Current-head CI and incremental reviews remain delivery gates.
- Manifest: `/private/tmp/compozy-issue-1020-pi-native-tools-20261029-041707-883357-lab/qa-artifacts/qa/bootstrap-manifest.json`.

## Charter and method

Ada needs a Pi session to discover Compozy tools, inspect itself, execute a terminal command, and receive a daemon monitor wake. Sessions enter through the public CLI. Both authentication policies use real Pi, pi-acp, hosted MCP, daemon, terminal execution, timer scheduling, and local model inference.

The controlled integration uses a lab-only Pi tool, `bridge_probe`, selected by the real Qwen model. It invokes the four requested native operations through Pi's actual nested-tool pipeline, reading their descriptors first, then ends the initial turn. It does not mock any transport or daemon operation. This deterministic integration is distinct from the unassisted persona attempts below.

## Observed results

| Lane | Session | Evidence |
| --- | --- | --- |
| Native CLI | `sess-ce103f3eec544d19` | 244 registered native tools; tool search, session status, terminal execution and monitor creation succeeded. Terminal exit 0, output `background-ready`. |
| Bound secret | `sess-b1fc7c475fcab634` | Same 244 tools and successful four calls. Uses dummy local Ollama credential, not an operator secret. |
| Unavailable bridge | `sess-ef09167ba113b95c`, repeated as `sess-108e5530b18764f0` on reviewed binary | Real Pi with `--no-mcp`: durable `provider_error.code=pi_native_tools_connection_failed`; CLI/HTTP/UDS guidance present, native router and skill catalog absent; model turn completes. |

The native-CLI initial turn `turn-d5736fe10af0461b` ended at 05:14:33Z; monitor `mon-cb220e785e42` produced `turn-53358d0d83070763`, ending at 05:15:04Z. Bound-secret initial turn `turn-8ed47d0faf124300` ended at 05:17:04Z; monitor `mon-2a309c4e3cf3` produced `turn-bc34eecac36ef270`, ending at 05:18:10Z. Both second turns carry `scheduler_reason=monitor_timer_elapsed` and preserve their session/ACP binding. The model did not reliably produce the requested exact acknowledgment wording; the persisted new turns prove the wake.

Retained evidence under the manifest's `qa/` directory:

- `probe-1793250872062.json`, `probe4-events.json`: native CLI calls and wake.
- `probe-1793251024244.json`, `bound-events.json`: bound-secret calls and wake.
- `broken-warning.json`, `reviewed-broken-events.json`, `reviewed-fallback-guidance.json`: failure-path re-walk.
- `transport-proof/result.json`: installed real pi-acp accepts a fixture server in ACP `session/new` but Pi lists no tools before the bridge; after loading the embedded extension it lists `mcp__compozy_probe__compozy__probe`. This fixture proves transport behavior, separately from live daemon evidence.

## Failures and repairs

The 1.7B free-form walk (`sess-c49f16abd99e845d`) used bash/sleep and claimed a wake without a monitor. The 4B retry (`sess-fba048fe3e08bb52`) reached real tool listing and attempted terminal execution but repeatedly guessed names/arguments; stopped after 20 minutes. Neither is recorded as a persona pass.

The first controlled run exposed inline-command override precedence and startup-warning identity/persistence defects. Production fixes consume the inline Pi override before wrapping it and use the existing typed provider-error payload with a durable event identity. Both lanes and the failure path were re-run successfully.

Greptile findings repaired ACP post-acceptance error classification, authored-prompt recovery, persisted prompt-overlay restoration, and repair of startup-only warning history. CodeRabbit's finding moved native availability filtering onto bundled native descriptors so custom CLI workflows survive. Session replacement also commits tool availability with its startup definition.

Focused owning race suites passed for launch isolation, extension lifecycle, stale readiness/cancellation, first prompt delivery, existing auth policies, rollback, command precedence, durable diagnostics, acceptance errors, recovery overlays, custom fallback instructions, and transcript repair. The reviewed binary built and ran the real unavailable-bridge journey. No full local race/E2E lane or `make gate` ran under the assignment. CI diagnosed one whitespace issue, now fixed. Targeted local lint timed out after reporting zero issues and is not green evidence.

## Isolation and limits

The native-CLI lane deliberately models an operator-owned Pi directory inside the lab provider home. It does not validate the operator's real OAuth login/refresh. An initial lab environment inherited provider-related variables and its usage collector ran; subsequent launches used an environment allowlist and disabled collection. No operator credentials were copied or used for inference, and no operator daemon/session was changed.

The strict bootstrap auditor was run and failed against the lab's default broad feature contract: it requires four actors, daemon Tasks, Web/API cross-surface work, artifact reuse, and a full local gate. Those are outside this targeted assignment (which explicitly forbids the full local gate). The audit is retained unchanged; this report does not claim a release-grade audit pass. The unassisted persona limitation also remains explicit.

Windows execution and remote/paid providers are not covered by this local lab. CI and reviewer status are reported on the PR.

The lab teardown completed with `TEARDOWN_CLEAN=true` and `TEARDOWN_ALL_CLEAN=true`; `qa/teardown.json` records the verified daemon, local Ollama, and extension processes stopped. No lab processes remain.
