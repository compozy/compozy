# QA Run Report — 2026-10-29 — Pi native tools

- Scope: issue #1020, native tool discovery, execution, monitor wake, and unavailable-bridge guidance.
- Cadence: targeted runtime/provider journey.
- Build: working branch `fix/issue-1020-pi-native-tools`.
- Environment: isolated Compozy home and socket, HTTP port 58004; pi 1.4.0 and pi-acp 0.0.31. Local Ollama/Qwen3 1.7B and 4B, with its own port and model store; no operator credentials.
- Status: in-progress.
- Manifest: `/private/tmp/compozy-issue-1020-pi-native-tools-20261029-041707-883357-lab/qa-artifacts/qa/bootstrap-manifest.json`.

## Session matrix

| Journey | Persona | Scope | Status |
| --- | --- | --- | --- |
| RT-pi-native-tools | Ada, CLI operator | Discover tools, inspect current session, run a terminal command, schedule and receive a monitor wake | Pending |
| RT-pi-native-tools | Ada, recovering operator | Unavailable extension produces typed warning and CLI guidance | Pending |

## Charter

Ada uses a local pi provider to run a background command and receive its result without polling. Enter through `compozy session new` and `session prompt`; inspect persisted history and monitor events through CLI after the agent finishes. Use an isolated native pi directory and repeat with bound-secret runtime materialization using a non-secret local-model credential. The adjacent canary is existing Claude/Codex provider launch tests. Bound the inference leg; record stalls rather than substitute a mock model.

## Evidence so far

- Real pi extension loading against a subprocess MCP fixture: one discoverable tool, readiness `ready`; this is transport evidence, not model or daemon-wake proof.
- Focused owning Go suites passed for launch isolation, readiness status, finalized startup prompt, fallback prompt composition, and existing provider auth policies.
- No full local gate or E2E lane was run; the assignment makes current-head CI the delivery gate.

## Session debriefs

- Native free-form walk, `sess-c49f16abd99e845d`: the 1.7B model used `bash`/`sleep` and claimed a wake without creating a monitor. Failed; no wake proof.
- Clean free-form retry, `sess-fba048fe3e08bb52`: the 4B model reached real native `tool_list` and attempted terminal execution, but repeatedly guessed names and arguments instead of discovering descriptors. Stopped after a bounded 20-minute attempt; the requested four-tool/wake journey remains unverified.
- Transport reproduction: the installed real pi-acp accepts a fixture MCP server in `session/new`, but Pi lists no Compozy tools without the bridge. With the embedded extension it lists `mcp__compozy_probe__compozy__probe`. Evidence: `qa/transport-proof/result.json`; this is transport evidence using an MCP I/O fixture, not a daemon-wake claim.
- The first controlled integration attempt exposed two implementation defects: inline `PI_ACP_PI_COMMAND` bypassed the wrapper, and a startup warning had no required transcript turn identity. The owning regression also exposed that `Raw` was not persisted by the canonical event codec. Repairs consume the inline override before launch and persist the warning through the existing typed `provider_error` field with a lifecycle event identity. Re-walk pending.
- Greptile identified post-acceptance error classification and recovery reassembly defects. Both are repaired in production and covered by the existing ACP acceptance and session transition suites.

## Isolation and evidence boundaries

The native-cli lane deliberately models an operator-owned Pi directory under the lab's provider home, as requested for credential isolation. It does not claim validation of the operator's real login or OAuth refresh. An initial lab environment inherited provider-related variables and its usage collector ran; subsequent launches use an environment allowlist and disabled collection. No operator credentials were copied or used for model inference.

The controlled probe is a lab-only Pi tool that invokes the actual native tools through Pi's nested-tool pipeline; its model is the real local Qwen provider. It does not replace the free-form persona walk or mock the daemon, MCP connection, terminal, or monitor. Its results will be recorded separately.

Focused race tests passed for launch, extension lifecycle, startup delivery, existing auth policy, runtime rollback, inline command precedence, durable diagnostics, post-acceptance failures, and authored-prompt recovery. Targeted lint reported zero issues but exited on the shared-host timeout, so it is not green evidence.

## Limits

Windows execution, remote providers, and independent reviewer feedback remain unverified.
