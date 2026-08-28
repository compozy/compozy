# BUG-20260827-native-agent-create-profile: Native workspace agent creation loses the active profile

- **Status:** fixed, pending remediation commit
- **Impact (user-side):** Blocks-Completion
- **Severity:** Critical · **Priority:** P0
- **Persona Affected:** Ada
- **Journey Step:** J-18 Author an agent with reusable runtime controls, structured create entry
- **Scenarios:** RT-029
- **Found:** 2026-08-27 · **Report:** docs/qa/reports/2026-08-27-acp-runtime-catalog.md

## Summary

`compozy__agent_create` rejected every valid workspace-scoped payload with `schema_invalid`, while the equivalent Web, CLI, and HTTP create surfaces succeeded.

## Reproduction

- **Charter:** CH-agent-runtime-default-options · **Tour:** Feature Tour
- **Environment:** desktop / wifi-fast / en-US; isolated local daemon with the default profile and workspace

1. Inspect `compozy__agent_create` and use a payload accepted by its published schema.
2. Set `scope=workspace`, provide the current workspace, and include a valid agent name and prompt.
3. Invoke the tool through `compozy tool invoke`.

**Expected:** The tool resolves the caller's active profile, writes the workspace AGENT.md, and returns the created agent.
**Actual:** The tool returned `tool_invalid_input` with `schema_invalid`; even the minimal required payload failed.

## Evidence

- `/Users/pedronauck/dev/qa-labs/compozy-acp-runtime-catalog-20260828-004625-083662-lab/qa-artifacts/qa/evidence/native-agent-create-before-fix.json`
- Equivalent HTTP and CLI requests created workspace agents successfully in the same isolated runtime.

## Fix

- **Root cause:** the native tool resolved the workspace without forwarding the caller's profile name. The profile-aware workspace resolver received an empty profile and the generic native error mapper surfaced that failure as `schema_invalid`.
- **Fix commit:** pending; included in the single remediation commit
- **Regression test:** `TestNativeAgentCreate/Should report the registered target workspace` now requires the active profile name at the canonical native-tool boundary.

## Verification

- **Retested:** 2026-08-27 in the same isolated lab after rebuilding and restarting the daemon
- **Result:** pass — `compozy__agent_create` created `native_opus_writer` with Cursor `claude-opus-5`, `xhigh`, Fast, and `thinking=true`; focused `-race` suite passed 11 tests.
- **Evidence:** `/Users/pedronauck/dev/qa-labs/compozy-acp-runtime-catalog-20260828-004625-083662-lab/qa-artifacts/qa/evidence/native-agent-create-after-fix.json`
