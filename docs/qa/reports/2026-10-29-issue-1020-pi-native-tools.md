# QA Run Report — 2026-10-29 — Pi native tools

- Scope: issue #1020, native tool discovery, execution, monitor wake, and unavailable-bridge guidance.
- Cadence: targeted runtime/provider journey.
- Build: working branch `fix/issue-1020-pi-native-tools`.
- Environment: isolated Compozy home and socket, HTTP port 58004; pi 1.4.0 and pi-acp 0.0.31. Local Ollama/Qwen3 1.7B, with its own port and model store; no operator credentials.
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

Pending.

## Limits

Windows execution, remote providers, and independent reviewer feedback remain unverified.
